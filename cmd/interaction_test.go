package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInteractionWaitReturnsSuccessExitCodeForPositiveOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
		extra  string
	}{
		{name: "approved", status: "approved", extra: `,"action":"approve"`},
		{name: "yes", status: "yes", extra: `,"action":"yes"`},
		{name: "replied", status: "replied", extra: `,"text":"ship it"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			t.Setenv("BEAM_TOKEN", "env_token")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/hooks/env_token/events/evt_test" {
					t.Fatalf("path = %q", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true,"event":{"id":"evt_test","body":"Deploy?","delivered":1,"response":{"status":"` + tc.status + `"` + tc.extra + `,"expiresAt":"2026-07-27T17:00:00Z"},"createdAt":"2026-07-27T17:00:00Z"}}`))
			}))
			defer server.Close()
			t.Setenv("BEAM_API_URL", server.URL)

			root := newRootCmd("test")
			root.SetArgs([]string{"interaction", "wait", "evt_test", "--timeout", "1s", "--poll", "1ms"})
			var out bytes.Buffer
			root.SetOut(&out)

			if err := root.Execute(); err != nil {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(out.String(), `"status":"`+tc.status+`"`) {
				t.Fatalf("output = %s", out.String())
			}
		})
	}
}

func TestInteractionWaitReturnsExpiredExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token/events/evt_test" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"event":{"id":"evt_test","body":"Deploy?","delivered":1,"response":{"status":"expired","expiresAt":"2026-07-27T17:00:00Z"},"createdAt":"2026-07-27T17:00:00Z"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"interaction", "wait", "evt_test", "--timeout", "1s", "--poll", "1ms"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrInteractionUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 4 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"status":"expired"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestInteractionWaitReturnsCanceledExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token/events/evt_test" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"event":{"id":"evt_test","body":"Deploy?","delivered":1,"response":{"status":"canceled","expiresAt":"2026-07-27T17:00:00Z"},"createdAt":"2026-07-27T17:00:00Z"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"interaction", "wait", "evt_test", "--timeout", "1s", "--poll", "1ms"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrInteractionUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 4 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"status":"canceled"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestInteractionWaitReturnsNoExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"event":{"id":"evt_test","body":"Deploy?","delivered":1,"response":{"status":"no","action":"no","expiresAt":"2026-07-27T17:00:00Z"},"createdAt":"2026-07-27T17:00:00Z"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"interaction", "wait", "evt_test", "--timeout", "1s", "--poll", "1ms"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrInteractionDenied) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 5 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"status":"no"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestInteractionWaitReturnsDeniedExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"event":{"id":"evt_test","body":"Deploy?","delivered":1,"response":{"status":"denied","expiresAt":"2026-07-27T17:00:00Z"},"createdAt":"2026-07-27T17:00:00Z"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"interaction", "wait", "evt_test", "--timeout", "1s", "--poll", "1ms"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if !errors.Is(err, ErrInteractionDenied) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 5 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
	if !strings.Contains(out.String(), `"status":"denied"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunAPIErrorKeepsStdoutEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid payload","code":"invalid_payload"}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	var out, stderr bytes.Buffer
	code := Run("test", []string{"notify", "hello"}, strings.NewReader(""), &out, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
	if !strings.Contains(stderr.String(), "400 Bad Request") || !strings.Contains(stderr.String(), "invalid_payload") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
