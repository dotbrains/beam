package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotbrains/beam/internal/beam"
)

func TestProviderWorkerReturnsTokenSafeDiagnostics(t *testing.T) {
	server := httptest.NewServer(providerWorkerHandler("apns-worker", "worker_secret"))
	defer server.Close()

	body := `{
		"operation":"notification",
		"eventId":"evt_test",
		"notification":{"title":"Deploys","body":"shipped"},
		"devices":[{"deviceId":"dev_ios","pushToken":"notification_secret_123"}],
		"createdAt":"2026-07-29T00:00:00Z"
	}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/deliver", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer worker_secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw bytes.Buffer
	if _, err := raw.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.StatusCode, raw.String())
	}
	if strings.Contains(raw.String(), "notification_secret_123") || strings.Contains(raw.String(), "worker_secret") {
		t.Fatalf("worker response leaked secret: %s", raw.String())
	}
	var decoded struct {
		Diagnostics []beam.ProviderDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", decoded.Diagnostics)
	}
	diagnostic := decoded.Diagnostics[0]
	if diagnostic.Provider != "apns-worker" || diagnostic.Operation != "notification" || diagnostic.DeviceID != "dev_ios" || diagnostic.Status != "accepted" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestProviderWorkerRequiresBearerToken(t *testing.T) {
	server := httptest.NewServer(providerWorkerHandler("apns-worker", "worker_secret"))
	defer server.Close()

	resp, err := http.Post(server.URL+"/deliver", "application/json", strings.NewReader(`{"operation":"notification"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestProviderWorkerSkipsMissingPushToken(t *testing.T) {
	diagnostics := workerDiagnostics("apns-worker", workerDeliveryRequest{
		Operation: "notification",
		Devices:   []workerTargetDevice{{DeviceID: "dev_ios"}},
	})
	if len(diagnostics) != 1 || diagnostics[0].Status != "skipped" || diagnostics[0].Reason != "missing_push_token" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestProviderWorkerAcceptsPushToStartTokenForActivityStart(t *testing.T) {
	diagnostics := workerDiagnostics("apns-worker", workerDeliveryRequest{
		Operation: "activity_start",
		Devices:   []workerTargetDevice{{DeviceID: "dev_ios", PushToStartToken: "activity_secret_123"}},
	})
	if len(diagnostics) != 1 || diagnostics[0].Status != "accepted" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
