package beam

import (
	"bytes"
	"encoding/json"
	"io"
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

func TestServiceEventHistoryIsRecentScopedAndTokenSafe(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	first := createTestService(t, server.URL, "First")
	second := createTestService(t, server.URL, "Second")
	firstOld := sendPrompt(t, server.URL, first.Token, `{"body":"first old","response":{"type":"yes_no"}}`)
	firstNew := sendPrompt(t, server.URL, first.Token, `{"body":"first new","response":{"type":"yes_no"}}`)
	_ = sendPrompt(t, server.URL, second.Token, `{"body":"second","response":{"type":"yes_no"}}`)

	resp, err := http.Get(server.URL + "/api/services/" + first.Service.ID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history status = %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bodyBytes, []byte(first.Token)) || bytes.Contains(bodyBytes, []byte(second.Token)) {
		t.Fatalf("history leaked token: %s", string(bodyBytes))
	}
	var body struct {
		Events []Event `json:"events"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 2 {
		t.Fatalf("events = %#v", body.Events)
	}
	if body.Events[0].ID != firstNew || body.Events[1].ID != firstOld {
		t.Fatalf("unexpected order/scope: %#v", body.Events)
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

func TestNotificationRateLimitReturnsRetryHints(t *testing.T) {
	store := NewStore()
	store.RegisterService(Service{
		ID:    "svc_limited",
		Token: "limited_token",
		Title: "Limited",
		Limits: ServiceLimits{
			RequestsPerMinute: 1,
			MonthlyOperations: 10,
		},
	})
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	first := newNotifyRequest(t, server.URL, "limited_token", `{"body":"one"}`, "deploy-1")
	if resp, err := http.DefaultClient.Do(first); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("first status = %d", resp.StatusCode)
		}
	}

	replay := newNotifyRequest(t, server.URL, "limited_token", `{"body":"one"}`, "deploy-1")
	if resp, err := http.DefaultClient.Do(replay); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("replay status = %d", resp.StatusCode)
		}
	}

	limited := newNotifyRequest(t, server.URL, "limited_token", `{"body":"two"}`, "")
	resp, err := http.DefaultClient.Do(limited)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatalf("missing Retry-After header")
	}
	var body struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		Code       string `json:"code"`
		Limit      int    `json:"limit"`
		RetryAfter int    `json:"retryAfter"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.OK || body.Error != ErrRateLimited.Error() || body.Code != "rate_limit" || body.Limit != 1 || body.RetryAfter <= 0 {
		t.Fatalf("unexpected rate limit body: %#v", body)
	}
}

func TestLiveActivityWritesShareMonthlyAllowance(t *testing.T) {
	store := NewStore()
	store.RegisterService(Service{
		ID:    "svc_allowance",
		Token: "allowance_token",
		Title: "Allowance",
		Limits: ServiceLimits{
			RequestsPerMinute: 10,
			MonthlyOperations: 1,
		},
	})
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	startResp, err := http.Post(server.URL+"/hooks/allowance_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Deploy","status":"Building","key":"deploy"}`))
	if err != nil {
		t.Fatal(err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", startResp.StatusCode)
	}

	notifyResp, err := http.Post(server.URL+"/hooks/allowance_token", "application/json", bytes.NewBufferString(`{"body":"after activity"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer notifyResp.Body.Close()
	if notifyResp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("notify status = %d", notifyResp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(notifyResp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != ErrAllowanceExceeded.Error() || body.Code != "monthly_allowance" || body.Limit != 1 {
		t.Fatalf("unexpected allowance body: %#v", body)
	}
}

func TestExpiredIdempotencyRecordCanBeReused(t *testing.T) {
	store := NewStore()
	first, _, err := store.SendNotification("dev_token", NotificationRequest{Body: "old"}, "deploy-1", `{"body":"old"}`)
	if err != nil {
		t.Fatal(err)
	}
	store.idempotency["dev_token:deploy-1"] = IdempotencyRecord{
		Fingerprint: `{"body":"old"}`,
		EventID:     first.ID,
		CreatedAt:   time.Now().UTC().Add(-25 * time.Hour),
	}

	second, idempotent, err := store.SendNotification("dev_token", NotificationRequest{Body: "new"}, "deploy-1", `{"body":"new"}`)
	if err != nil {
		t.Fatal(err)
	}
	if idempotent {
		t.Fatalf("expired idempotency key replayed old response")
	}
	if second.ID == first.ID || second.Body != "new" {
		t.Fatalf("unexpected event after key reuse: %#v", second)
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

func newNotifyRequest(t *testing.T, baseURL, token, body, idempotencyKey string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/hooks/"+token, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return req
}
