package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationEventRetainsProviderDiagnostics(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"diagnostics","deviceIds":["dev_local"]}`))
	if err != nil {
		t.Fatal(err)
	}
	_, eventID := decodeEventID(t, resp)
	readResp, err := http.Get(server.URL + "/hooks/dev_token/events/" + eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer readResp.Body.Close()
	var body struct {
		Event Event `json:"event"`
	}
	if err := json.NewDecoder(readResp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	diagnostics := body.Event.ProviderDiagnostics
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].Provider != "local" || diagnostics[0].Operation != "notification" || diagnostics[0].DeviceID != "dev_local" || diagnostics[0].Status != "accepted" {
		t.Fatalf("diagnostic = %#v", diagnostics[0])
	}
}

func TestActivityResponsesExposeProviderDiagnostics(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Build","status":"Running","deviceIds":["dev_local"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", resp.StatusCode)
	}
	var body struct {
		Failed              int                  `json:"failed"`
		ProviderDiagnostics []ProviderDiagnostic `json:"providerDiagnostics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Failed != 0 || len(body.ProviderDiagnostics) != 1 {
		t.Fatalf("activity response = %#v", body)
	}
	diagnostic := body.ProviderDiagnostics[0]
	if diagnostic.Provider != "local" || diagnostic.Operation != "activity_start" || diagnostic.DeviceID != "dev_local" || diagnostic.Status != "accepted" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}
