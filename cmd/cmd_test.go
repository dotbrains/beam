package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecute_Version(t *testing.T) {
	os.Args = []string{"beam", "--version"}
	err := Execute("0.0.1-test")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestNewRootCmd(t *testing.T) {
	root := newRootCmd("0.1.0")
	if root.Use != "beam" {
		t.Errorf("Use = %q", root.Use)
	}

	// Verify subcommands.
	cmds := make(map[string]bool)
	for _, c := range root.Commands() {
		cmds[c.Name()] = true
	}
	for _, want := range []string{"config", "services"} {
		if !cmds[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestNewRootCmd_Version(t *testing.T) {
	root := newRootCmd("1.2.3")
	if root.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", root.Version)
	}
}

func TestExecute_Help(t *testing.T) {
	root := newRootCmd("dev")
	root.SetArgs([]string{"--help"})
	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "beam") {
		t.Error("expected project name in help output")
	}
	if !strings.Contains(output, "config") {
		t.Error("expected 'config' subcommand in help")
	}
}

func TestAccessLog_RedactsCredentialPaths(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := accessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), logger)
	req := httptest.NewRequest(http.MethodPost, "/hooks/secret_token/events/evt_123?token=ignored", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	handler.ServeHTTP(httptest.NewRecorder(), req)

	output := logs.String()
	if strings.Contains(output, "secret_token") {
		t.Fatalf("log leaked webhook token: %s", output)
	}
	if !strings.Contains(output, `"/hooks/:token/events/evt_123"`) {
		t.Fatalf("log did not include redacted path: %s", output)
	}
	if !strings.Contains(output, `"status":202`) {
		t.Fatalf("log did not include response status: %s", output)
	}
}

func TestRedactRequestPath_RedactsDeviceCodes(t *testing.T) {
	got := redactRequestPath("/api/auth/device/adc_secret/token")
	if got != "/api/auth/device/:deviceCode/token" {
		t.Fatalf("redacted path = %q", got)
	}
}

func TestRunConfigInit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	root := newRootCmd("test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "init"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	// Config file should exist.
	configPath := filepath.Join(tmp, ".config", "beam", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file not created")
	}

	assertConfigInitOutput(t, buf.Bytes())
}

func TestRunConfigInit_AlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Pre-create config.
	configDir := filepath.Join(tmp, ".config", "beam")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd("test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "init"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when config exists")
	}
}

func TestRunWritesDiagnosticsToStderr(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configDir := filepath.Join(tmp, ".config", "beam")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, stderr bytes.Buffer
	code := Run("test", []string{"config", "init"}, strings.NewReader(""), &out, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q", out.String())
	}
	if !strings.Contains(stderr.String(), "beam: config already exists") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunMapsCobraUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown command", args: []string{"missing"}, want: "unknown command"},
		{name: "unknown flag", args: []string{"config", "init", "--missing"}, want: "unknown flag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			code := Run("test", tt.args, strings.NewReader(""), &out, &stderr)

			if code != 2 {
				t.Fatalf("exit code = %d", code)
			}
			if out.Len() != 0 {
				t.Fatalf("stdout = %q", out.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunConfigInit_Force(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Pre-create config.
	configDir := filepath.Join(tmp, ".config", "beam")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd("test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "init", "--force"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	assertConfigInitOutput(t, buf.Bytes())
}

func TestAPIClientEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_API_URL", "http://127.0.0.1:9090")
	t.Setenv("BEAM_TOKEN", "env_token")

	cfg, _, err := apiClient()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.APIURL != "http://127.0.0.1:9090" {
		t.Fatalf("APIURL = %q", cfg.APIURL)
	}
	if cfg.Token != "env_token" {
		t.Fatalf("Token = %q", cfg.Token)
	}
}

func TestAPIClientSendsBearerToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	cfg, client, err := apiClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := getJSON(client, apiURL(cfg, "/api/services")); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer beam_agent_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestServicesEventsCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/svc_test/events" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"events":[{"id":"evt_test","body":"done","delivered":1}]}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "events", "svc_test"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer beam_agent_test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(out.String(), `"events":[{"body":"done","delivered":1,"id":"evt_test"}]`) {
		t.Fatalf("output = %s", out.String())
	}
	if strings.Contains(out.String(), "beam_agent_test") {
		t.Fatalf("output leaked token: %s", out.String())
	}
}

func TestNotifyReadsStdinAndDeviceFlags(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")

	var gotBody struct {
		Body      string   `json:"body"`
		DeviceIDs []string `json:"deviceIds"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/env_token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":1}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"notify", "--stdin", "--device", "dev_local", "--device", "dev_remote"})
	root.SetIn(strings.NewReader("hello from stdin\n"))
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if gotBody.Body != "hello from stdin" {
		t.Fatalf("body = %q", gotBody.Body)
	}
	if strings.Join(gotBody.DeviceIDs, ",") != "dev_local,dev_remote" {
		t.Fatalf("DeviceIDs = %#v", gotBody.DeviceIDs)
	}
	if !strings.Contains(out.String(), `"delivered":1`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestNotifyNoDeviceAccepted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":0}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"notify", "hello"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"delivered":0`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestNotifyStrictNoDeviceAccepted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "env_token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"eventId":"evt_test","delivered":0}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"notify", "hello", "--strict"})
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

func TestExitCodeMapsErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: 0},
		{name: "auth", err: APIError{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized"}, want: 3},
		{name: "api", err: APIError{StatusCode: http.StatusBadRequest, Status: "400 Bad Request"}, want: 1},
		{name: "network", err: NetworkError{Err: errors.New("dial tcp refused")}, want: 6},
		{name: "device", err: ErrNoDeviceAccepted, want: 7},
		{name: "usage", err: UsageError{Err: errors.New("missing arg")}, want: 2},
		{name: "timeout", err: ErrInteractionTimedOut, want: 4},
		{name: "denied", err: ErrInteractionDenied, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode = %d, want %d", got, tt.want)
			}
		})
	}
}

func assertConfigInitOutput(t *testing.T, data []byte) {
	t.Helper()
	var body struct {
		OK      bool   `json:"ok"`
		Path    string `json:"path"`
		Created bool   `json:"created"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("output is not JSON: %s", data)
	}
	if !body.OK || body.Path == "" || !body.Created {
		t.Fatalf("unexpected config init output: %#v", body)
	}
}
