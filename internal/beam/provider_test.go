package beam

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestNotificationUsesConfiguredPushProvider(t *testing.T) {
	store := NewStoreWithProvider(fakePushProvider{
		notificationDiagnostics: []ProviderDiagnostic{{
			Provider:  "fake",
			Operation: "notification",
			DeviceID:  "dev_local",
			Status:    "failed",
			Reason:    "provider_rejected",
			CreatedAt: time.Now().UTC(),
		}},
	})
	event, _, err := store.SendNotification("dev_token", NotificationRequest{Body: "provider", DeviceIDs: []string{"dev_local"}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Delivered != 0 || len(event.ProviderDiagnostics) != 1 {
		t.Fatalf("event = %#v", event)
	}
	diagnostic := event.ProviderDiagnostics[0]
	if diagnostic.Provider != "fake" || diagnostic.Status != "failed" || diagnostic.Reason != "provider_rejected" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestActivityUsesConfiguredPushProvider(t *testing.T) {
	store := NewStoreWithProvider(fakePushProvider{
		activityDiagnostics: []ProviderDiagnostic{{
			Provider:  "fake",
			Operation: "activity_start",
			DeviceID:  "dev_local",
			Status:    "accepted",
			CreatedAt: time.Now().UTC(),
		}},
	})
	activity, _, err := store.StartActivity("dev_token", ActivityRequest{Title: "Deploy", Status: "Running", DeviceIDs: []string{"dev_local"}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if activity.Delivered != 1 || len(activity.ProviderDiagnostics) != 1 {
		t.Fatalf("activity = %#v", activity)
	}
	if activity.ProviderDiagnostics[0].Provider != "fake" {
		t.Fatalf("diagnostic = %#v", activity.ProviderDiagnostics[0])
	}
}

func TestNotificationProviderFailureReturnsBadGateway(t *testing.T) {
	server := httptest.NewServer(Handler(NewStoreWithProvider(fakePushProvider{err: ErrProviderFailure})))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"provider down"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assertProviderFailureResponse(t, resp)
}

func TestActivityProviderFailureReturnsBadGateway(t *testing.T) {
	server := httptest.NewServer(Handler(NewStoreWithProvider(fakePushProvider{err: ErrProviderFailure})))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Build","status":"Running"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assertProviderFailureResponse(t, resp)
}

func TestNotificationRejectsInvalidDeviceIDLists(t *testing.T) {
	assertValidationField(t, validateNotification(NotificationRequest{
		Body:      "deploy",
		DeviceIDs: []string{},
	}), "deviceIds")
	assertValidationField(t, validateNotification(NotificationRequest{
		Body:      "deploy",
		DeviceIDs: []string{"dev_local", "dev_local"},
	}), "deviceIds")
}

func TestActivityStartRejectsInvalidDeviceIDLists(t *testing.T) {
	assertValidationField(t, validateActivityStart(ActivityRequest{
		Title:     "Deploy",
		Status:    "Running",
		DeviceIDs: []string{},
	}), "deviceIds")
	assertValidationField(t, validateActivityStart(ActivityRequest{
		Title:     "Deploy",
		Status:    "Running",
		DeviceIDs: []string{"dev_local", "dev_local"},
	}), "deviceIds")
}

func assertProviderFailureResponse(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.OK || body.Error != ErrProviderFailure.Error() || body.Code != "provider_failure" {
		t.Fatalf("provider failure response = %#v", body)
	}
}

func assertValidationField(t *testing.T, err error, field string) {
	t.Helper()
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	for _, item := range validationErr.Fields {
		if item.Field == field {
			return
		}
	}
	t.Fatalf("missing field error %q in %#v", field, validationErr.Fields)
}

type fakePushProvider struct {
	notificationDiagnostics []ProviderDiagnostic
	activityDiagnostics     []ProviderDiagnostic
	err                     error
}

func (p fakePushProvider) SendNotification(req PushNotification) ([]ProviderDiagnostic, error) {
	return p.notificationDiagnostics, p.err
}

func (p fakePushProvider) StartActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return p.activityDiagnostics, p.err
}

func (p fakePushProvider) UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return p.activityDiagnostics, p.err
}

func (p fakePushProvider) EndActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return p.activityDiagnostics, p.err
}
