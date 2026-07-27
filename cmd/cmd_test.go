package cmd

import (
	"bytes"
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
	for _, want := range []string{"config"} {
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

	if !strings.Contains(buf.String(), "Wrote default config") {
		t.Error("expected success message")
	}
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

	if !strings.Contains(buf.String(), "Wrote default config") {
		t.Error("expected success message with --force")
	}
}
