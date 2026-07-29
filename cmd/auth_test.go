package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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
	t.Setenv("BEAM_API_URL", "https://beam.example.com")

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
	if !strings.Contains(out.String(), `"apiUrl":"https://beam.example.com"`) {
		t.Fatalf("status output = %s", out.String())
	}
}

func TestAuthLoginDeviceFlow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	var openedURL string
	t.Cleanup(func(original func(string) error) func() {
		return func() { openBrowser = original }
	}(openBrowser))
	openBrowser = func(url string) error {
		openedURL = url
		return nil
	}
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
	if openedURL != "https://beam.example.com/auth/verify" {
		t.Fatalf("opened URL = %q", openedURL)
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

func TestAuthLoginDeviceFlowContinuesWhenBrowserOpenFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Cleanup(func(original func(string) error) func() {
		return func() { openBrowser = original }
	}(openBrowser))
	openBrowser = func(url string) error {
		return errors.New("no browser")
	}
	expiresAt := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/device":
			_, _ = w.Write([]byte(`{"ok":true,"device":{"deviceCode":"adc_test","userCode":"ABCD-1234","verifyUrl":"https://beam.example.com/auth/verify","expiresAt":"` + expiresAt + `"}}`))
		case "/api/auth/device/adc_test/token":
			_, _ = w.Write([]byte(`{"ok":true,"status":"approved","token":"beam_agent_test","scopes":["notify"],"clientName":"CI","expiresAt":"` + expiresAt + `"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs([]string{"auth", "login", "--timeout", "1s", "--poll", "1ms"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"authenticated":true`) {
		t.Fatalf("login output = %s", out.String())
	}
	if !strings.Contains(stderr.String(), "Could not open browser: no browser") {
		t.Fatalf("stderr = %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Open https://beam.example.com/auth/verify and enter code ABCD-1234") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestAuthLogoutRevoke(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	revoked := false
	var gotAuth string
	var gotBody struct {
		Token string `json:"token"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/auth/revoke" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
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
	if gotAuth != "Bearer beam_agent_test" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotBody.Token != "beam_agent_test" {
		t.Fatalf("revoke token = %q", gotBody.Token)
	}
	if !strings.Contains(out.String(), `"configured":false`) {
		t.Fatalf("logout output = %s", out.String())
	}
	if strings.Contains(out.String(), "beam_agent_test") {
		t.Fatalf("logout output leaked token: %s", out.String())
	}
}

func TestAuthConnectionsListCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/connections" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"connections":[{"id":"adc_test","clientName":"CI","scopes":["auth"],"status":"approved"}]}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "connections", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer beam_agent_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(out.String(), `"connections":[{"clientName":"CI","id":"adc_test","scopes":["auth"],"status":"approved"}]`) {
		t.Fatalf("output = %s", out.String())
	}
	if strings.Contains(out.String(), "beam_agent_test") {
		t.Fatalf("output leaked token: %s", out.String())
	}
}

func TestAuthConnectionsRevokeCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/connections/adc_test/revoke" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"connection":{"id":"adc_test","clientName":"CI","scopes":["auth"],"status":"revoked"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"auth", "connections", "revoke", "adc_test"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer beam_agent_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(out.String(), `"connection":{"clientName":"CI","id":"adc_test","scopes":["auth"],"status":"revoked"}`) {
		t.Fatalf("output = %s", out.String())
	}
	if strings.Contains(out.String(), "beam_agent_test") {
		t.Fatalf("output leaked token: %s", out.String())
	}
}

func TestAuthConnectionsForbiddenReturnsAuthExitCode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/connections" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"error":"insufficient scope","code":"forbidden"}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	var out, stderr bytes.Buffer
	code := Run("test", []string{"auth", "connections", "list"}, strings.NewReader(""), &out, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
	if !strings.Contains(stderr.String(), "403 Forbidden") || !strings.Contains(stderr.String(), "forbidden") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
