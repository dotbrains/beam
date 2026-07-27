package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResponseAnswerSchedulesCallbackAttempts(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	created := createTestService(t, server.URL, "Approvals")
	eventID := sendPrompt(t, server.URL, created.Token, `{
		"body":"ship deploy?",
		"response":{
			"type":"approval",
			"correlationId":"deploy-42",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)

	resp, err := http.Post(server.URL+"/hooks/"+created.Token+"/events/"+eventID+"/respond", "application/json", bytes.NewBufferString(`{"action":"approve"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("respond status = %d", resp.StatusCode)
	}
	var body struct {
		Event Event `json:"event"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	response := body.Event.Response
	if response == nil {
		t.Fatalf("missing response: %#v", body.Event)
	}
	if response.Type != "approval" || response.Status != "approved" || response.Action != "approve" {
		t.Fatalf("unexpected response state: %#v", response)
	}
	if response.CorrelationID != "deploy-42" {
		t.Fatalf("correlation = %q", response.CorrelationID)
	}
	assertCallbackSchedule(t, eventID, response.CallbackAttempts)
}

func TestEventsFromAnotherServiceAreInvisible(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	first := createTestService(t, server.URL, "First")
	second := createTestService(t, server.URL, "Second")
	eventID := sendPrompt(t, server.URL, first.Token, `{"body":"scoped","response":{"type":"yes_no"}}`)

	readResp, err := http.Get(server.URL + "/hooks/" + second.Token + "/events/" + eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-service read status = %d", readResp.StatusCode)
	}

	respondResp, err := http.Post(server.URL+"/hooks/"+second.Token+"/events/"+eventID+"/respond", "application/json", bytes.NewBufferString(`{"action":"yes"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer respondResp.Body.Close()
	if respondResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-service respond status = %d", respondResp.StatusCode)
	}
}

func TestCanceledPromptDoesNotScheduleCallbackAttempts(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"cancel me",
		"response":{
			"type":"text",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)

	resp, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d", resp.StatusCode)
	}
	var body struct {
		Event Event `json:"event"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Event.Response == nil || body.Event.Response.Status != "canceled" {
		t.Fatalf("unexpected response: %#v", body.Event.Response)
	}
	if len(body.Event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", body.Event.Response.CallbackAttempts)
	}
}

func createTestService(t *testing.T, baseURL, title string) struct {
	Service PublicService `json:"service"`
	Token   string        `json:"token"`
} {
	t.Helper()
	resp, err := http.Post(baseURL+"/api/services", "application/json", bytes.NewBufferString(`{"title":"`+title+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created struct {
		Service PublicService `json:"service"`
		Token   string        `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	return created
}

func sendPrompt(t *testing.T, baseURL, token, payload string) string {
	t.Helper()
	resp, err := http.Post(baseURL+"/hooks/"+token, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send status = %d", resp.StatusCode)
	}
	var body struct {
		EventID string `json:"eventId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.EventID == "" {
		t.Fatalf("missing event id: %#v", body)
	}
	return body.EventID
}

func assertCallbackSchedule(t *testing.T, eventID string, attempts []CallbackAttempt) {
	t.Helper()
	if len(attempts) != 5 {
		t.Fatalf("attempts = %#v", attempts)
	}
	expected := []time.Duration{0, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, time.Hour}
	base := attempts[0].ScheduledAt
	for i, attempt := range attempts {
		if attempt.EventID != eventID || attempt.Attempt != i+1 || attempt.Status != "scheduled" {
			t.Fatalf("attempt[%d] = %#v", i, attempt)
		}
		if got := attempt.ScheduledAt.Sub(base); got != expected[i] {
			t.Fatalf("attempt[%d] delay = %s", i, got)
		}
	}
}
