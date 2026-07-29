package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
