package beam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDeliverDueCallbacksPostsSettledEvent(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()
	var gotAuth string
	var gotEventID string
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			EventID string `json:"eventId"`
			Event   Event  `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotEventID = body.EventID
		if body.Event.Response == nil || body.Event.Response.Status != "approved" {
			t.Fatalf("event response = %#v", body.Event.Response)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer callbackServer.Close()

	store.events["evt_callback"] = callbackEvent("evt_callback", callbackServer.URL, now)
	count, err := store.DeliverDueCallbacks(context.Background(), callbackServer.Client(), now)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	if gotAuth != "Bearer 0123456789abcdef" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotEventID != "evt_callback" {
		t.Fatalf("eventId = %q", gotEventID)
	}
	attempt := store.events["evt_callback"].Response.CallbackAttempts[0]
	if attempt.Status != "delivered" || attempt.StatusCode != http.StatusNoContent || attempt.DeliveredAt == nil {
		t.Fatalf("attempt = %#v", attempt)
	}
}

func TestDeliverDueCallbacksRecordsFailureAndKeepsFutureRetry(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()
	requests := 0
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "try later", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer callbackServer.Close()

	event := callbackEvent("evt_retry", callbackServer.URL, now)
	event.Response.CallbackAttempts[1].ScheduledAt = now.Add(time.Minute)
	store.events[event.ID] = event

	count, err := store.DeliverDueCallbacks(context.Background(), callbackServer.Client(), now)
	if err == nil {
		t.Fatal("expected delivery error")
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	attempts := store.events[event.ID].Response.CallbackAttempts
	if attempts[0].Status != "failed" || attempts[0].StatusCode != http.StatusBadGateway || attempts[0].Error == "" {
		t.Fatalf("attempt[0] = %#v", attempts[0])
	}
	if attempts[1].Status != "scheduled" || attempts[1].DeliveredAt != nil {
		t.Fatalf("attempt[1] = %#v", attempts[1])
	}

	count, err = store.DeliverDueCallbacks(context.Background(), callbackServer.Client(), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retry count = %d", count)
	}
	attempts = store.events[event.ID].Response.CallbackAttempts
	if attempts[1].Status != "delivered" || attempts[1].StatusCode != http.StatusNoContent || attempts[1].DeliveredAt == nil {
		t.Fatalf("attempt[1] after retry = %#v", attempts[1])
	}
	if requests != 2 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestMatchingIdempotencyKeyReturnsAcceptedWhileNotificationInFlight(t *testing.T) {
	provider := &blockingNotificationProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := httptest.NewServer(Handler(NewStoreWithProvider(provider)))
	defer server.Close()

	var first sync.WaitGroup
	first.Add(1)
	go func() {
		defer first.Done()
		req := newNotifyRequest(t, server.URL, "dev_token", `{"body":"deploy"}`, "deploy-1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("first status = %d", resp.StatusCode)
		}
	}()

	<-provider.started
	req := newNotifyRequest(t, server.URL, "dev_token", `{"body":"deploy"}`, "deploy-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("replay while in flight status = %d", resp.StatusCode)
	}
	close(provider.release)
	first.Wait()
}

func TestMatchingIdempotencyKeyReturnsAcceptedWhileActivityStartInFlight(t *testing.T) {
	provider := &blockingActivityProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := httptest.NewServer(Handler(NewStoreWithProvider(provider)))
	defer server.Close()

	var first sync.WaitGroup
	first.Add(1)
	go func() {
		defer first.Done()
		req := newActivityRequest(t, server.URL, `{"key":"deploy","title":"Deploy","status":"Running"}`, "deploy-1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("first status = %d", resp.StatusCode)
		}
	}()

	<-provider.started
	req := newActivityRequest(t, server.URL, `{"key":"deploy","title":"Deploy","status":"Running"}`, "deploy-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("replay while activity start in flight status = %d", resp.StatusCode)
	}
	close(provider.release)
	first.Wait()
}

func TestDuplicateActivityStartConflictsWhileProviderInFlight(t *testing.T) {
	provider := &blockingActivityProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := httptest.NewServer(Handler(NewStoreWithProvider(provider)))
	defer server.Close()

	var first sync.WaitGroup
	first.Add(1)
	go func() {
		defer first.Done()
		req := newActivityRequest(t, server.URL, `{"key":"deploy","title":"Deploy","status":"Running"}`, "")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("first status = %d", resp.StatusCode)
		}
	}()

	<-provider.started
	req := newActivityRequest(t, server.URL, `{"key":"deploy","title":"Deploy","status":"Running"}`, "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate while activity start in flight status = %d", resp.StatusCode)
	}
	close(provider.release)
	first.Wait()
}

func TestHTTPPushProviderPostsTokenSafeDeliveryRequest(t *testing.T) {
	var gotAuth string
	var got providerDeliveryRequest
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"diagnostics":[{"provider":"apns-worker","operation":"notification","deviceId":"dev_local","status":"accepted","createdAt":"2026-07-29T00:00:00Z"}]}`))
	}))
	defer providerServer.Close()

	provider := HTTPPushProvider{Endpoint: providerServer.URL, Token: "provider_secret", Client: providerServer.Client()}
	diagnostics, err := provider.SendNotification(PushNotification{
		EventID:   "evt_test",
		DeviceIDs: []string{"dev_local"},
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer provider_secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if got.Operation != "notification" || got.EventID != "evt_test" || len(got.DeviceIDs) != 1 || got.DeviceIDs[0] != "dev_local" {
		t.Fatalf("provider request = %#v", got)
	}
	if got.ActivityID != "" {
		t.Fatalf("notification request included activityId: %#v", got)
	}
	if len(diagnostics) != 1 || diagnostics[0].Provider != "apns-worker" || diagnostics[0].Status != "accepted" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestHTTPPushProviderReturnsProviderFailureWithoutLeakingToken(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider_secret rejected", http.StatusBadGateway)
	}))
	defer providerServer.Close()

	provider := HTTPPushProvider{Endpoint: providerServer.URL, Token: "provider_secret", Client: providerServer.Client()}
	_, err := provider.StartActivity(ActivityPush{
		ActivityID: "act_test",
		DeviceIDs:  []string{"dev_local"},
		CreatedAt:  time.Now().UTC(),
	})
	if !errors.Is(err, ErrProviderFailure) {
		t.Fatalf("err = %v, want provider failure", err)
	}
	if strings.Contains(err.Error(), "provider_secret") {
		t.Fatalf("provider error leaked token: %v", err)
	}
}

func callbackEvent(id, callbackURL string, now time.Time) Event {
	return Event{
		ID:        id,
		ServiceID: "svc_dev",
		Title:     "Beam",
		Body:      "ship?",
		Delivered: 1,
		CreatedAt: now,
		Response: &ResponseState{
			Type:          "approval",
			Status:        "approved",
			Action:        "approve",
			CorrelationID: "deploy-42",
			ExpiresAt:     now.Add(time.Hour),
			RespondedAt:   &now,
			CallbackURL:   callbackURL,
			CallbackToken: "0123456789abcdef",
			CallbackAttempts: []CallbackAttempt{
				{EventID: id, Attempt: 1, Status: "scheduled", ScheduledAt: now},
				{EventID: id, Attempt: 2, Status: "scheduled", ScheduledAt: now.Add(30 * time.Second)},
			},
		},
	}
}

type blockingNotificationProvider struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *blockingNotificationProvider) SendNotification(req PushNotification) ([]ProviderDiagnostic, error) {
	p.once.Do(func() { close(p.started) })
	<-p.release
	return LocalPushProvider{}.SendNotification(req)
}

func (p *blockingNotificationProvider) StartActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return LocalPushProvider{}.StartActivity(req)
}

func (p *blockingNotificationProvider) UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return LocalPushProvider{}.UpdateActivity(req)
}

func (p *blockingNotificationProvider) EndActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return LocalPushProvider{}.EndActivity(req)
}

type blockingActivityProvider struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *blockingActivityProvider) SendNotification(req PushNotification) ([]ProviderDiagnostic, error) {
	return LocalPushProvider{}.SendNotification(req)
}

func (p *blockingActivityProvider) StartActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	p.once.Do(func() { close(p.started) })
	<-p.release
	return LocalPushProvider{}.StartActivity(req)
}

func (p *blockingActivityProvider) UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return LocalPushProvider{}.UpdateActivity(req)
}

func (p *blockingActivityProvider) EndActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return LocalPushProvider{}.EndActivity(req)
}

func newActivityRequest(t *testing.T, baseURL, body, idempotencyKey string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/hooks/dev_token/live-activities", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return req
}
