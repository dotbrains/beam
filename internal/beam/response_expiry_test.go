package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExpiredPromptDoesNotScheduleCallbackAttempts(t *testing.T) {
	store := NewStore()
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"expire me",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)
	event := store.events[eventID]
	event.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	store.events[eventID] = event

	resp, err := http.Get(server.URL + "/hooks/dev_token/events/" + eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d", resp.StatusCode)
	}
	var body struct {
		Event Event `json:"event"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Event.Response == nil || body.Event.Response.Status != "expired" {
		t.Fatalf("unexpected response: %#v", body.Event.Response)
	}
	if len(body.Event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", body.Event.Response.CallbackAttempts)
	}
}

func TestExpiredPromptRejectsLateResponseWithoutCallbacks(t *testing.T) {
	store := NewStore()
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"too late",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)
	event := store.events[eventID]
	event.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	store.events[eventID] = event

	resp, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/respond", "application/json", bytes.NewBufferString(`{"action":"approve"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("respond status = %d", resp.StatusCode)
	}

	event = store.events[eventID]
	if event.Response == nil || event.Response.Status != "expired" {
		t.Fatalf("unexpected response: %#v", event.Response)
	}
	if len(event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", event.Response.CallbackAttempts)
	}
}

func TestExpiredPromptRejectsCancelWithoutCallbacks(t *testing.T) {
	store := NewStore()
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"too late",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)
	event := store.events[eventID]
	event.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	store.events[eventID] = event

	resp, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/cancel", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cancel status = %d", resp.StatusCode)
	}

	event = store.events[eventID]
	if event.Response == nil || event.Response.Status != "expired" {
		t.Fatalf("unexpected response: %#v", event.Response)
	}
	if len(event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", event.Response.CallbackAttempts)
	}
}

func TestPublicHTTPSValidationRejectsEmbeddedCredentials(t *testing.T) {
	handler := Handler(NewStore())

	req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
		"body":"credentials",
		"imageUrl":"https://user:pass@example.com/avatar.png",
		"response":{
			"type":"approval",
			"callback":{"url":"https://user:pass@example.com/beam","token":"0123456789abcdef"}
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	var body struct {
		Fields []FieldError `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !hasFieldError(body.Fields, "imageUrl") || !hasFieldError(body.Fields, "response.callback.url") {
		t.Fatalf("field errors = %#v", body.Fields)
	}
}

func TestCallbackTokenRejectsWhitespace(t *testing.T) {
	handler := Handler(NewStore())

	req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
		"body":"credentials",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"                "}
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	assertFieldError(t, resp.Body, "response.callback.token")
}

func hasFieldError(fields []FieldError, want string) bool {
	for _, field := range fields {
		if field.Field == want {
			return true
		}
	}
	return false
}
