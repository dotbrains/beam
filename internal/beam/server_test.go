package beam

import (
	"bytes"
	"encoding/json"
	"io"
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

func TestNotificationValidationRejectsPrivateImageURL(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"build passed","imageUrl":"https://127.0.0.1/avatar.png"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "imageUrl")
}

func TestNotificationValidationRejectsInvalidResponseExpiry(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"approve deploy","response":{"type":"approval","expiresInSeconds":10}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "response.expiresInSeconds")
}

func TestLiveActivityRejectsEmptyUpdate(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "activity")
}

func TestLiveActivitySequenceConflict(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{"status":"Testing","ifSequence":2}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLiveActivityRejectsUpdateAfterEnd(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	resp, err := http.Post(server.URL+"/hooks/dev_token/live-activities/deploy/end", "application/json", bytes.NewBufferString(`{"status":"Done"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("end status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{"status":"Again"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func startActivity(t *testing.T, baseURL string) {
	t.Helper()
	resp, err := http.Post(baseURL+"/hooks/dev_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Deploy","status":"Building","key":"deploy"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", resp.StatusCode)
	}
}

func assertFieldError(t *testing.T, body io.Reader, field string) {
	t.Helper()
	var payload struct {
		Fields []FieldError `json:"fields"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	for _, item := range payload.Fields {
		if item.Field == field {
			return
		}
	}
	t.Fatalf("missing field error %q in %#v", field, payload.Fields)
}
