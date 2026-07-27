package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
