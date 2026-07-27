package cmd

import (
	"bytes"
	"encoding/json"
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
