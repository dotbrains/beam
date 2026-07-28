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

func TestMetricsEndpointReportsDeliveryAndCallbackCounts(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{
		"body":"metrics",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	body, eventID := decodeEventID(t, resp)
	if eventID == "" {
		t.Fatalf("missing event id: %s", body)
	}

	respond, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/respond", "application/json", bytes.NewBufferString(`{"action":"approve"}`))
	if err != nil {
		t.Fatal(err)
	}
	respond.Body.Close()
	if respond.StatusCode != http.StatusOK {
		t.Fatalf("respond status = %d", respond.StatusCode)
	}

	metrics := readMetrics(t, server.URL)
	for _, want := range []string{
		"beam_deliveries_total 1",
		"beam_callback_attempts_scheduled_total 5",
	} {
		if !bytes.Contains(metrics, []byte(want)) {
			t.Fatalf("metrics missing %q: %s", want, metrics)
		}
	}
}

func TestNotificationWithNoRegisteredDevicesReturnsMessage(t *testing.T) {
	store := NewStore()
	store.RegisterService(Service{ID: "svc_empty", Token: "empty_token", Title: "Empty"})
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/empty_token", "application/json", bytes.NewBufferString(`{"body":"no devices"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("notify status = %d", resp.StatusCode)
	}
	var body struct {
		OK        bool   `json:"ok"`
		EventID   string `json:"eventId"`
		Delivered int    `json:"delivered"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.EventID == "" || body.Delivered != 0 || body.Message == "" {
		t.Fatalf("unexpected no-device response: %#v", body)
	}
}

func TestProviderFailureReturnsBadGateway(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeStoreError(recorder, ErrProviderFailure)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.OK || body.Error != ErrProviderFailure.Error() || body.Code != "provider_failure" {
		t.Fatalf("unexpected provider failure body: %#v", body)
	}
}

func TestProviderFailureIncrementsMetrics(t *testing.T) {
	server := httptest.NewServer(Handler(NewStoreWithProvider(fakePushProvider{err: ErrProviderFailure})))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"provider down"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	metrics := readMetrics(t, server.URL)
	if !bytes.Contains(metrics, []byte("beam_provider_failures_total 1")) {
		t.Fatalf("provider failure metric missing: %s", metrics)
	}
}

func TestRateLimitIncrementsMetrics(t *testing.T) {
	store := NewStore()
	store.RegisterService(Service{
		ID:    "svc_limited_metrics",
		Token: "limited_metrics_token",
		Title: "Limited Metrics",
		Limits: ServiceLimits{
			RequestsPerMinute: 1,
			MonthlyOperations: 10,
		},
	})
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	first, err := http.Post(server.URL+"/hooks/limited_metrics_token", "application/json", bytes.NewBufferString(`{"body":"one"}`))
	if err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d", first.StatusCode)
	}
	limited, err := http.Post(server.URL+"/hooks/limited_metrics_token", "application/json", bytes.NewBufferString(`{"body":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	limited.Body.Close()
	if limited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d", limited.StatusCode)
	}

	metrics := readMetrics(t, server.URL)
	if !bytes.Contains(metrics, []byte("beam_http_rate_limited_responses_total 1")) {
		t.Fatalf("rate limit metric missing: %s", metrics)
	}
}

func TestInFlightNotificationIdempotencyReturnsAccepted(t *testing.T) {
	store := NewStore()
	body := `{"body":"deploy"}`
	store.idempotency[hashToken("dev_token")+":deploy-1"] = IdempotencyRecord{Fingerprint: body, CreatedAt: time.Now().UTC()}
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/hooks/dev_token", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "deploy-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestBlankNotificationIdempotencyKeyReturnsBadRequest(t *testing.T) {
	handler := Handler(NewStore())

	req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{"body":"deploy"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", " ")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	assertFieldError(t, resp.Body, "Idempotency-Key")
}

func TestInFlightActivityIdempotencyReturnsAccepted(t *testing.T) {
	store := NewStore()
	body := `{"title":"Deploy","status":"Running"}`
	store.idempotency[hashToken("dev_token")+":activity:deploy-1"] = IdempotencyRecord{Fingerprint: body, CreatedAt: time.Now().UTC()}
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/hooks/dev_token/live-activities", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "deploy-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestBlankActivityIdempotencyKeyReturnsBadRequest(t *testing.T) {
	handler := Handler(NewStore())

	req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token/live-activities", bytes.NewBufferString(`{"title":"Deploy","status":"Running"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", " ")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	assertFieldError(t, resp.Body, "Idempotency-Key")
}

func TestCompletedActivityIdempotencyReplayReturnsOK(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	body := `{"title":"Deploy","status":"Running"}`
	first := postActivityWithIdempotency(t, server.URL, body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d", first.StatusCode)
	}
	firstBody := decodeActivityReplay(t, first)
	replay := postActivityWithIdempotency(t, server.URL, body)
	if replay.StatusCode != http.StatusOK {
		t.Fatalf("replay status = %d", replay.StatusCode)
	}
	replayBody := decodeActivityReplay(t, replay)
	if !replayBody.Idempotent || replayBody.ActivityID != firstBody.ActivityID {
		t.Fatalf("unexpected replay body: %#v after %#v", replayBody, firstBody)
	}
}

func decodeEventID(t *testing.T, resp *http.Response) ([]byte, string) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("notify status = %d body = %s", resp.StatusCode, body)
	}
	var payload struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	return body, payload.EventID
}

func postActivityWithIdempotency(t *testing.T, baseURL, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/hooks/dev_token/live-activities", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "deploy-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeActivityReplay(t *testing.T, resp *http.Response) struct {
	ActivityID string `json:"activityId"`
	Idempotent bool   `json:"idempotent"`
} {
	t.Helper()
	defer resp.Body.Close()
	var body struct {
		ActivityID string `json:"activityId"`
		Idempotent bool   `json:"idempotent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func readMetrics(t *testing.T, baseURL string) []byte {
	t.Helper()
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
