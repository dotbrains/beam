package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dotbrains/beam/internal/beam"
)

func TestSQLiteStorePersistsEventsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	event, _, err := store.SendNotification("dev_token", beam.NotificationRequest{Body: "build passed"}, "build-1", `{"body":"build passed"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Event("dev_token", event.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != event.ID || got.Body != "build passed" {
		t.Fatalf("unexpected event after reopen: %#v", got)
	}

	replayed, idempotent, err := reopened.SendNotification("dev_token", beam.NotificationRequest{Body: "build passed"}, "build-1", `{"body":"build passed"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !idempotent || replayed.ID != event.ID {
		t.Fatalf("expected idempotent replay of %s, got %#v idempotent=%v", event.ID, replayed, idempotent)
	}
}

func TestSQLiteStorePersistsProviderFailureDiagnostics(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	store.store = beam.NewStoreWithProvider(failingProvider{})
	if _, _, err := store.SendNotification("dev_token", beam.NotificationRequest{Body: "provider down"}, "", ""); !errors.Is(err, beam.ErrProviderFailure) {
		t.Fatalf("err = %v, want provider failure", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err := reopened.ServiceEvents("svc_dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || len(events[0].ProviderDiagnostics) != 1 {
		t.Fatalf("events = %#v", events)
	}
	diagnostic := events[0].ProviderDiagnostics[0]
	if diagnostic.Status != "failed" || diagnostic.Reason != "provider_failure" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestSQLiteStorePersistsExpiredLateResponse(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	event, _, err := store.SendNotification("dev_token", beam.NotificationRequest{
		Body: "approve deploy",
		Response: &beam.ResponseRequest{
			Type: "approval",
			Callback: &beam.CallbackRequest{
				URL:   "https://callbacks.example.com/beam",
				Token: "0123456789abcdef",
			},
		},
	}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := store.store.Snapshot()
	expired := snapshot.Events[event.ID]
	expired.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	snapshot.Events[event.ID] = expired
	store.store = beam.NewStoreFromSnapshot(snapshot)

	if _, err := store.RespondEvent("dev_token", event.ID, beam.ResponseAnswerRequest{Action: "approve"}); !errors.Is(err, beam.ErrNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Event("dev_token", event.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Response == nil || got.Response.Status != "expired" {
		t.Fatalf("unexpected response after reopen: %#v", got.Response)
	}
	if len(got.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts after reopen: %#v", got.Response.CallbackAttempts)
	}
}

func TestSQLiteStorePersistsActivitiesAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	activity, _, err := store.StartActivity("dev_token", beam.ActivityRequest{Title: "Deploy", Status: "Building", Key: "deploy"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateActivity("dev_token", "deploy", beam.ActivityRequest{Status: "Testing"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Activity("dev_token", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != activity.ID || got.Sequence != 1 || got.State.Status != "Testing" {
		t.Fatalf("unexpected activity after reopen: %#v", got)
	}
}

type failingProvider struct{}

func (failingProvider) SendNotification(beam.PushNotification) ([]beam.ProviderDiagnostic, error) {
	return nil, beam.ErrProviderFailure
}

func (failingProvider) StartActivity(beam.ActivityPush) ([]beam.ProviderDiagnostic, error) {
	return nil, beam.ErrProviderFailure
}

func (failingProvider) UpdateActivity(beam.ActivityPush) ([]beam.ProviderDiagnostic, error) {
	return nil, beam.ErrProviderFailure
}

func (failingProvider) EndActivity(beam.ActivityPush) ([]beam.ProviderDiagnostic, error) {
	return nil, beam.ErrProviderFailure
}

func TestSQLiteStorePersistsServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateService(beam.ServiceCreateRequest{Title: "CI"})
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := store.RotateServiceToken(created.Service.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	service, err := reopened.Service(created.Service.ID)
	if err != nil {
		t.Fatal(err)
	}
	if service.Title != "CI" {
		t.Fatalf("unexpected service after reopen: %#v", service)
	}
	if _, _, err := reopened.SendNotification(created.Token, beam.NotificationRequest{Body: "old"}, "", ""); err != beam.ErrUnknownWebhook {
		t.Fatalf("expected old token to be revoked, got %v", err)
	}
	if _, _, err := reopened.SendNotification(rotated.Token, beam.NotificationRequest{Body: "new"}, "", ""); err != nil {
		t.Fatal(err)
	}

	payload := snapshotPayload(t, dbPath)
	if strings.Contains(payload, created.Token) || strings.Contains(payload, rotated.Token) {
		t.Fatalf("snapshot contains plaintext service token: %s", payload)
	}
	if !strings.Contains(payload, `"tokenHash"`) {
		t.Fatalf("snapshot is missing token hash: %s", payload)
	}
}

func TestSQLiteStorePersistsDevices(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateService(beam.ServiceCreateRequest{Title: "CI"})
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.RegisterDevice(created.Service.ID, beam.DeviceRegisterRequest{
		Name:             "Nick's iPhone",
		Platform:         "ios",
		PushToStartToken: "0123456789abcdef",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !device.PushToStartTokenRegistered {
		t.Fatalf("push token registration missing: %#v", device)
	}
	if _, err := store.DeactivateDevice(created.Service.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	devices, err := reopened.Devices(created.Service.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].ID != device.ID || devices[0].Active || !devices[0].PushToStartTokenRegistered {
		t.Fatalf("unexpected devices after reopen: %#v", devices)
	}
}

func TestSQLiteStorePersistsAuthConnectionRevocation(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	device, err := store.StartAuthDevice(beam.AuthDeviceRequest{ClientName: "Agent", Scopes: []string{"notify"}}, "https://beam.example.com")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := store.ApproveAuthDevice(device.UserCode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RevokeAuthDevice(approved.DeviceCode); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	connections := reopened.AuthDevices()
	if len(connections) != 1 || connections[0].Status != "revoked" {
		t.Fatalf("connections after reopen: %#v", connections)
	}
	if tokenDevice, err := reopened.AuthDeviceToken(approved.DeviceCode); err != nil || tokenDevice.Token != "" {
		t.Fatalf("unexpected token after reopen: %#v err=%v", tokenDevice, err)
	}
}

func snapshotPayload(t *testing.T, dbPath string) string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var payload string
	if err := db.QueryRow(`SELECT payload FROM snapshots WHERE name = 'beam'`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
