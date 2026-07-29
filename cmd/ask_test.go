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

func TestNotifyAskAliasSendsPrompt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var got struct {
		Body     string `json:"body"`
		Response struct {
			Type             string `json:"type"`
			ExpiresInSeconds int    `json:"expiresInSeconds"`
		} `json:"response"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"notify", "ask", "Deploy?", "--approval", "--expires-in", "2m"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Body != "Deploy?" {
		t.Fatalf("body = %q", got.Body)
	}
	if got.Response.Type != "approval" {
		t.Fatalf("type = %q", got.Response.Type)
	}
	if got.Response.ExpiresInSeconds != 120 {
		t.Fatalf("expiresInSeconds = %d", got.Response.ExpiresInSeconds)
	}
	if !strings.Contains(out.String(), `"eventId":"evt_test"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestAskSendsCallbackAndCorrelation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var got struct {
		Body     string `json:"body"`
		Response struct {
			Type          string `json:"type"`
			CorrelationID string `json:"correlationId"`
			Callback      struct {
				URL   string `json:"url"`
				Token string `json:"token"`
			} `json:"callback"`
		} `json:"response"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending","correlationId":"deploy-42"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{
		"ask", "Deploy?", "--text",
		"--correlation-id", "deploy-42",
		"--callback-url", "https://callbacks.example.com/beam",
		"--callback-token", "0123456789abcdef",
	})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Body != "Deploy?" || got.Response.Type != "text" {
		t.Fatalf("request = %#v", got)
	}
	if got.Response.CorrelationID != "deploy-42" {
		t.Fatalf("correlationId = %q", got.Response.CorrelationID)
	}
	if got.Response.Callback.URL != "https://callbacks.example.com/beam" || got.Response.Callback.Token != "0123456789abcdef" {
		t.Fatalf("callback = %#v", got.Response.Callback)
	}
	if !strings.Contains(out.String(), `"correlationId":"deploy-42"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestNotifyAskSendsNotificationRoutingFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var got struct {
		Body      string   `json:"body"`
		Title     string   `json:"title"`
		ImageURL  string   `json:"imageUrl"`
		URL       string   `json:"url"`
		DeviceIDs []string `json:"deviceIds"`
		Response  struct {
			Type string `json:"type"`
		} `json:"response"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{
		"notify", "ask", "Deploy?",
		"--approval",
		"--title", "Deploy Bot",
		"--image", "https://beam.example.com/avatar.png",
		"--url", "https://beam.example.com/deploys/42",
		"--device", "dev_phone",
		"--device", "dev_watch",
	})
	root.SetOut(&bytes.Buffer{})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Body != "Deploy?" || got.Title != "Deploy Bot" {
		t.Fatalf("request = %#v", got)
	}
	if got.ImageURL != "https://beam.example.com/avatar.png" {
		t.Fatalf("imageUrl = %q", got.ImageURL)
	}
	if got.URL != "https://beam.example.com/deploys/42" {
		t.Fatalf("url = %q", got.URL)
	}
	if strings.Join(got.DeviceIDs, ",") != "dev_phone,dev_watch" {
		t.Fatalf("deviceIds = %#v", got.DeviceIDs)
	}
	if got.Response.Type != "approval" {
		t.Fatalf("response type = %q", got.Response.Type)
	}
}

func TestNotifySendsPresentationRoutingAndIdempotencyFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var gotKey string
	var got struct {
		Body      string   `json:"body"`
		Title     string   `json:"title"`
		ImageURL  string   `json:"imageUrl"`
		URL       string   `json:"url"`
		DeviceIDs []string `json:"deviceIds"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_notify","delivered":1}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{
		"notify", "Deploy shipped",
		"--title", "Deploy Bot",
		"--image", "https://beam.example.com/avatar.png",
		"--url", "https://beam.example.com/deploys/42",
		"--device", "dev_phone",
		"--device", "dev_watch",
		"--idempotency-key", "deploy-42",
	})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotKey != "deploy-42" {
		t.Fatalf("Idempotency-Key = %q", gotKey)
	}
	if got.Body != "Deploy shipped" || got.Title != "Deploy Bot" {
		t.Fatalf("request = %#v", got)
	}
	if got.ImageURL != "https://beam.example.com/avatar.png" || got.URL != "https://beam.example.com/deploys/42" {
		t.Fatalf("request = %#v", got)
	}
	if strings.Join(got.DeviceIDs, ",") != "dev_phone,dev_watch" {
		t.Fatalf("deviceIds = %#v", got.DeviceIDs)
	}
	if !strings.Contains(out.String(), `"eventId":"evt_notify"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestAskReadsPromptFromStdin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var got struct {
		Body     string `json:"body"`
		Response struct {
			Type string `json:"type"`
		} `json:"response"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_stdin","delivered":1,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"ask", "--stdin", "--yes-no"})
	root.SetIn(strings.NewReader("Deploy from pipeline?\n"))
	root.SetOut(&bytes.Buffer{})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Body != "Deploy from pipeline?" {
		t.Fatalf("body = %q", got.Body)
	}
	if got.Response.Type != "yes_no" {
		t.Fatalf("response type = %q", got.Response.Type)
	}
}

func TestAskRejectsEmptyStdinPrompt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	root := newRootCmd("test")
	root.SetArgs([]string{"ask", "--stdin", "--approval"})
	root.SetIn(strings.NewReader("\n"))
	root.SetOut(&bytes.Buffer{})

	err := root.Execute()
	var usageErr UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error = %v, want usage error", err)
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
}

func TestNotifyAskSendsIdempotencyKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotKey = r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"notify", "ask", "Deploy?", "--approval", "--idempotency-key", "deploy-42"})
	root.SetOut(&bytes.Buffer{})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotKey != "deploy-42" {
		t.Fatalf("Idempotency-Key = %q", gotKey)
	}
}

func TestAskTimeoutBackfillsExpiryForCompatibility(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var expires int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got struct {
			Response struct {
				ExpiresInSeconds int `json:"expiresInSeconds"`
			} `json:"response"`
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		expires = got.Response.ExpiresInSeconds
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"ask", "Ship?", "--yes-no", "--timeout", "3m"})
	root.SetOut(&bytes.Buffer{})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if expires != 180 {
		t.Fatalf("expiresInSeconds = %d", expires)
	}
}

func TestAskExpiresInOverridesTimeoutExpiry(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var expires int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got struct {
			Response struct {
				ExpiresInSeconds int `json:"expiresInSeconds"`
			} `json:"response"`
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		expires = got.Response.ExpiresInSeconds
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"ask", "Ship?", "--yes-no", "--timeout", "3m", "--expires-in", "5m"})
	root.SetOut(&bytes.Buffer{})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if expires != 300 {
		t.Fatalf("expiresInSeconds = %d", expires)
	}
}

func TestAskWaitReturnsTimeoutExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/hooks/env_token":
			_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1,"response":{"status":"pending"}}`))
		case "/hooks/env_token/events/evt_test":
			_, _ = w.Write([]byte(`{"ok":true,"event":{"id":"evt_test","body":"Ship?","delivered":1,"response":{"status":"pending","expiresAt":"2026-07-27T17:00:00Z"},"createdAt":"2026-07-27T17:00:00Z"}}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"ask", "Ship?", "--yes-no", "--wait", "--timeout", "1ns", "--poll", "1s"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrInteractionTimedOut) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 4 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"status":"pending"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestAskStrictReturnsNoDeviceExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":0,"response":{"status":"pending"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"ask", "Ship?", "--yes-no", "--strict"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrNoDeviceAccepted) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 7 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"delivered":0`) {
		t.Fatalf("output = %s", out.String())
	}
}
