package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActivityListCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token/live-activities" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activities":[{"id":"act_test","key":"deploy","sequence":0,"status":"active"}]}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "list"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"activities":[`) {
		t.Fatalf("output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"key":"deploy"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestActivityGetCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/hooks/env_token/live-activities/deploy" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","key":"deploy","sequence":2,"status":"active"}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "get", "deploy"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"activityId":"act_test"`) || !strings.Contains(out.String(), `"sequence":2`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestActivityStartCommandSendsDevices(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	var got struct {
		DeviceIDs []string `json:"deviceIds"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token/live-activities" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":2}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "start", "--title", "Deploy", "--status", "Running", "--device", "dev_a", "--device", "dev_b"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.DeviceIDs, ",") != "dev_a,dev_b" {
		t.Fatalf("deviceIds = %#v", got.DeviceIDs)
	}
}

func TestActivityStartCommandSendsAdvancedFlags(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	var got struct {
		Title            string   `json:"title"`
		Status           string   `json:"status"`
		Detail           *string  `json:"detail"`
		Progress         *float64 `json:"progress"`
		Symbol           string   `json:"symbol"`
		AccentColor      string   `json:"accentColor"`
		Style            string   `json:"style"`
		PrivacyMode      string   `json:"privacyMode"`
		Key              string   `json:"key"`
		DeviceIDs        []string `json:"deviceIds"`
		Replace          bool     `json:"replace"`
		ExpiresInSeconds int      `json:"expiresInSeconds"`
		StaleAfter       int      `json:"staleAfterSeconds"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token/live-activities" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Idempotency-Key") != "deploy-184" {
			t.Fatalf("Idempotency-Key = %q", r.Header.Get("Idempotency-Key"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":1}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{
		"activity", "start",
		"--title", "Deploy #184",
		"--status", "Building",
		"--detail", "Step 1",
		"--progress", "0.5",
		"--symbol", "shippingbox",
		"--accent-color", "#00ffaa",
		"--style", "ring",
		"--privacy-mode", "private",
		"--key", "deploy",
		"--device", "dev_a",
		"--replace",
		"--expires-in", "2h",
		"--stale-after", "30m",
		"--idempotency-key", "deploy-184",
	})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Deploy #184" || got.Status != "Building" || got.Key != "deploy" {
		t.Fatalf("request = %#v", got)
	}
	if got.Detail == nil || *got.Detail != "Step 1" {
		t.Fatalf("detail = %#v", got.Detail)
	}
	if got.Progress == nil || *got.Progress != 0.5 {
		t.Fatalf("progress = %#v", got.Progress)
	}
	if got.Symbol != "shippingbox" || got.AccentColor != "#00ffaa" {
		t.Fatalf("request = %#v", got)
	}
	if got.Style != "ring" || got.PrivacyMode != "private" {
		t.Fatalf("request = %#v", got)
	}
	if !got.Replace || strings.Join(got.DeviceIDs, ",") != "dev_a" {
		t.Fatalf("request = %#v", got)
	}
	if got.ExpiresInSeconds != 7200 || got.StaleAfter != 1800 {
		t.Fatalf("request = %#v", got)
	}
}

func TestActivityUpdateCommandSendsSequenceAndDetail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	var got struct {
		Status      string   `json:"status"`
		Detail      *string  `json:"detail"`
		Progress    *float64 `json:"progress"`
		IfSequence  *int     `json:"ifSequence"`
		StaleAfter  int      `json:"staleAfterSeconds"`
		DismissedIn int      `json:"dismissAfterSeconds"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/hooks/env_token/live-activities/deploy" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":1}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{
		"activity", "update", "deploy",
		"--status", "Testing",
		"--detail", "Running tests",
		"--progress", "0.75",
		"--if-sequence", "2",
		"--stale-after", "10m",
	})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Status != "Testing" || got.Detail == nil || *got.Detail != "Running tests" {
		t.Fatalf("request = %#v", got)
	}
	if got.Progress == nil || *got.Progress != 0.75 {
		t.Fatalf("progress = %#v", got.Progress)
	}
	if got.IfSequence == nil || *got.IfSequence != 2 {
		t.Fatalf("ifSequence = %#v", got.IfSequence)
	}
	if got.StaleAfter != 600 || got.DismissedIn != 0 {
		t.Fatalf("request = %#v", got)
	}
}

func TestActivityUpdateCommandCanClearDetailAndProgress(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/hooks/env_token/live-activities/deploy" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":1}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "update", "deploy", "--clear-detail", "--clear-progress"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if detail, ok := got["detail"]; !ok || detail != nil {
		t.Fatalf("detail = %#v, present=%v", detail, ok)
	}
	if progress, ok := got["progress"]; !ok || progress != nil {
		t.Fatalf("progress = %#v, present=%v", progress, ok)
	}
}

func TestActivityUpdateRejectsDismissAfterFlag(t *testing.T) {
	var out, stderr bytes.Buffer
	code := Run("test", []string{"activity", "update", "deploy", "--dismiss-after", "1h"}, strings.NewReader(""), &out, &stderr)

	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
	if !strings.Contains(stderr.String(), "unknown flag: --dismiss-after") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestActivityEndCommandSendsDismissAfterAndSequence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	var got struct {
		Status       string `json:"status"`
		IfSequence   *int   `json:"ifSequence"`
		DismissAfter int    `json:"dismissAfterSeconds"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/hooks/env_token/live-activities/deploy/end" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":1}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{
		"activity", "end", "deploy",
		"--status", "Shipped",
		"--if-sequence", "3",
		"--dismiss-after", "1h",
	})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Status != "Shipped" {
		t.Fatalf("status = %q", got.Status)
	}
	if got.IfSequence == nil || *got.IfSequence != 3 {
		t.Fatalf("ifSequence = %#v", got.IfSequence)
	}
	if got.DismissAfter != 3600 {
		t.Fatalf("dismissAfterSeconds = %d", got.DismissAfter)
	}
}

func TestActivityStartNoDeviceAccepted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":0}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "start", "--title", "Deploy", "--status", "Running"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"accepted":0`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestActivityStartStrictNoDeviceAccepted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":0}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "start", "--title", "Deploy", "--status", "Running", "--strict"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrNoDeviceAccepted) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 7 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"accepted":0`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestActivityUpdateNoDeviceAccepted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"activityId":"act_test","accepted":0}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"activity", "update", "act_test", "--status", "Running", "--strict"})

	err := root.Execute()
	if !errors.Is(err, ErrNoDeviceAccepted) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 7 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
}
