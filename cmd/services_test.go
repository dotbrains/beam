package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServicesCreatePrintsTokenOnce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")

	var got struct {
		Title    string `json:"title"`
		ImageURL string `json:"imageUrl"`
		URL      string `json:"url"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer beam_agent_test" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"service":{"id":"svc_test","title":"CI","url":"https://ci.example.com"},"token":"beam_webhook_secret"}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "create", "--title", "CI", "--image", "https://ci.example.com/icon.png", "--url", "https://ci.example.com"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Title != "CI" || got.ImageURL != "https://ci.example.com/icon.png" || got.URL != "https://ci.example.com" {
		t.Fatalf("request = %#v", got)
	}
	if !strings.Contains(out.String(), `"token":"beam_webhook_secret"`) {
		t.Fatalf("create output did not include one-time token: %s", out.String())
	}
}

func TestServicesListKeepsTokenSafeOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"services":[{"id":"svc_test","title":"CI","token":"beam_webhook_secret","tokenHash":"hash_only"}]}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "list"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "beam_webhook_secret") || strings.Contains(out.String(), `"token":`) {
		t.Fatalf("list output leaked token-like field: %s", out.String())
	}
	if !strings.Contains(out.String(), `"services":[`) {
		t.Fatalf("list output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"tokenHash":"hash_only"`) {
		t.Fatalf("list output removed token hash evidence: %s", out.String())
	}
}

func TestServicesShowKeepsTokenSafeOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services/svc_test" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":{"id":"svc_test","title":"CI","token":"beam_webhook_secret","tokenHash":"hash_only"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "show", "svc_test"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "beam_webhook_secret") || strings.Contains(out.String(), `"token":`) {
		t.Fatalf("show output leaked token-like field: %s", out.String())
	}
	if !strings.Contains(out.String(), `"tokenHash":"hash_only"`) {
		t.Fatalf("show output removed token hash evidence: %s", out.String())
	}
}

func TestServicesRotatePrintsTokenOnce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services/svc_test/rotate-token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":{"id":"svc_test","title":"CI","tokenHash":"new_hash"},"token":"beam_new_webhook_secret"}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "rotate-token", "svc_test"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"token":"beam_new_webhook_secret"`) {
		t.Fatalf("rotate output did not include one-time token: %s", out.String())
	}
}

func TestServicesUpdateCanClearDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")
	var got struct {
		ImageURL *string `json:"imageUrl"`
		URL      *string `json:"url"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services/svc_test" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer beam_agent_test" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"service":{"id":"svc_test","title":"CI"}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "update", "svc_test", "--image", "", "--url", ""})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.ImageURL == nil || *got.ImageURL != "" {
		t.Fatalf("imageUrl = %#v, want explicit empty string", got.ImageURL)
	}
	if got.URL == nil || *got.URL != "" {
		t.Fatalf("url = %#v, want explicit empty string", got.URL)
	}
}

func TestServicesDeviceRegisterHidesPushToStartToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")

	var got struct {
		Name             string `json:"name"`
		Platform         string `json:"platform"`
		PushToken        string `json:"pushToken"`
		PushToStartToken string `json:"pushToStartToken"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services/svc_test/devices" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true,"device":{"id":"dev_test","name":"Nick's iPhone","platform":"ios","active":true,"pushToken":"notify_secret","pushToStartToken":"push_secret","pushTokenRegistered":true,"pushToStartTokenRegistered":true}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "devices", "register", "svc_test", "--name", "Nick's iPhone", "--push-token", "notify_secret", "--push-to-start-token", "push_secret"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Nick's iPhone" || got.Platform != "ios" || got.PushToken != "notify_secret" || got.PushToStartToken != "push_secret" {
		t.Fatalf("request = %#v", got)
	}
	if strings.Contains(out.String(), "notify_secret") || strings.Contains(out.String(), "push_secret") {
		t.Fatalf("device output leaked push material: %s", out.String())
	}
	if !strings.Contains(out.String(), `"pushTokenRegistered":true`) {
		t.Fatalf("device output = %s", out.String())
	}
	if !strings.Contains(out.String(), `"pushToStartTokenRegistered":true`) {
		t.Fatalf("device output = %s", out.String())
	}
}

func TestServicesDeviceDeactivateHidesPushTokens(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("BEAM_TOKEN", "beam_agent_test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/services/svc_test/devices/dev_test/deactivate" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"device":{"id":"dev_test","name":"Nick's iPhone","active":false,"pushToken":"notify_secret","pushToStartToken":"push_secret","pushTokenRegistered":true,"pushToStartTokenRegistered":true}}`))
	}))
	defer server.Close()
	t.Setenv("BEAM_API_URL", server.URL)

	root := newRootCmd("test")
	root.SetArgs([]string{"services", "devices", "deactivate", "svc_test", "dev_test"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "notify_secret") || strings.Contains(out.String(), "push_secret") {
		t.Fatalf("deactivate output leaked push material: %s", out.String())
	}
	if !strings.Contains(out.String(), `"pushTokenRegistered":true`) || !strings.Contains(out.String(), `"pushToStartTokenRegistered":true`) {
		t.Fatalf("deactivate output removed token-safe registration state: %s", out.String())
	}
}

func TestOpenPushProviderRequiresHTTPProviderURL(t *testing.T) {
	if _, err := openPushProvider("http", "", "provider_secret"); err == nil {
		t.Fatal("expected missing provider URL error")
	}
	if provider, err := openPushProvider("http", "https://push.example.com/deliver", "provider_secret"); err != nil || provider == nil {
		t.Fatalf("provider = %#v err = %v", provider, err)
	}
}
