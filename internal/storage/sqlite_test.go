package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
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

func TestSQLiteStoreRecordsAppliedMigrations(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	assertMigrationVersion(t, dbPath, 1)

	reopened, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	assertMigrationVersion(t, dbPath, 1)
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

func TestSQLiteStorePersistsExpiredCancel(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "beam.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	event, _, err := store.SendNotification("dev_token", beam.NotificationRequest{
		Body: "cancel deploy",
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

	if _, err := store.CancelEvent("dev_token", event.ID); !errors.Is(err, beam.ErrNotFound) {
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

func assertMigrationVersion(t *testing.T, dbPath string, version int) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration %d count = %d, want 1", version, count)
	}
}
