package beam

import (
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
