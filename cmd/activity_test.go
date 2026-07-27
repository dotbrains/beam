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
	root.SetArgs([]string{"activity", "update", "act_test", "--status", "Running"})

	err := root.Execute()
	if !errors.Is(err, ErrNoDeviceAccepted) {
		t.Fatalf("error = %v", err)
	}
	if ExitCode(err) != 7 {
		t.Fatalf("ExitCode = %d", ExitCode(err))
	}
}
