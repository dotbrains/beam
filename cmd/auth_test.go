package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
