package storage

import (
	"context"
	"path/filepath"
	"testing"

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
}
