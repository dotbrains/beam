package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationEndpoint(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"build passed","title":"CI"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true || body["eventId"] == "" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestNotificationIdempotencyConflict(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	first, err := http.NewRequest(http.MethodPost, server.URL+"/hooks/dev_token", bytes.NewBufferString(`{"body":"one"}`))
	if err != nil {
		t.Fatal(err)
	}
	first.Header.Set("Content-Type", "application/json")
	first.Header.Set("Idempotency-Key", "deploy-1")
	if resp, err := http.DefaultClient.Do(first); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}

	second, err := http.NewRequest(http.MethodPost, server.URL+"/hooks/dev_token", bytes.NewBufferString(`{"body":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	second.Header.Set("Content-Type", "application/json")
	second.Header.Set("Idempotency-Key", "deploy-1")
	resp, err := http.DefaultClient.Do(second)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLiveActivityLifecycle(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Deploy","status":"Building","key":"deploy"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{"status":"Testing","progress":0.6}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["sequence"].(float64) != 1 {
		t.Fatalf("unexpected sequence: %#v", body)
	}
}
