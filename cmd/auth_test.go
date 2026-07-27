package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthLoginStatusAndLogout(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{
		"auth", "login",
		"--token", "beam_test",
		"--scope", "notify",
		"--scope", "activity",
		"--client-name", "CI",
		"--expires-in", "1h",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"authenticated":true`) {
		t.Fatalf("login output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"clientName":"CI"`) {
		t.Fatalf("login output = %s", out.String())
	}
	cfgPath := filepath.Join(tmp, ".config", "beam", "config.yaml")
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 0600", got)
	}

	out.Reset()
	root = newRootCmd("test")
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"source":"config"`) {
		t.Fatalf("status output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"configured":true`) {
		t.Fatalf("status output = %s", out.String())
	}

	out.Reset()
	root = newRootCmd("test")
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "logout"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"authenticated":false`) {
		t.Fatalf("logout output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"configured":false`) {
		t.Fatalf("logout output = %s", out.String())
	}
}

func TestAuthStatusUsesEnvToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"source":"env"`) {
		t.Fatalf("status output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"authenticated":true`) {
		t.Fatalf("status output = %s", out.String())
	}
}

func TestAuthLoginDeviceFlow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	polls := 0
	expiresAt := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/device":
			_, _ = w.Write([]byte(`{"ok":true,"device":{"deviceCode":"adc_test","userCode":"ABCD-1234","verifyUrl":"https://beam.example.com/auth/verify","expiresAt":"` + expiresAt + `"}}`))
		case "/api/auth/device/adc_test/token":
			polls++
			if polls == 1 {
				_, _ = w.Write([]byte(`{"ok":true,"status":"pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"status":"approved","token":"beam_agent_test","scopes":["notify"],"clientName":"CI","expiresAt":"` + expiresAt + `"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "login", "--client-name", "CI", "--scope", "notify", "--timeout", "1s", "--poll", "1ms"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"authenticated":true`) {
		t.Fatalf("login output = %s", out.String())
	}
	if polls != 2 {
		t.Fatalf("polls = %d", polls)
	}

	root = newRootCmd("test")
	out.Reset()
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"configured":true`) {
		t.Fatalf("status output = %s", out.String())
	}
}

func TestAuthLogoutRevoke(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	revoked := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/revoke" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		revoked = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"credential":{"status":"revoked"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"auth", "login", "--token", "beam_agent_test"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	root = newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "logout", "--revoke"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("expected revoke endpoint to be called")
	}
	if !strings.Contains(out.String(), `"configured":false`) {
		t.Fatalf("logout output = %s", out.String())
	}
}
