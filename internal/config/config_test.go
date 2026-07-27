package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
}

func TestConfigDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir error: %v", err)
	}
	if dir != filepath.Join(tmp, ".config", "beam") {
		t.Errorf("ConfigDir = %q", dir)
	}
}

func TestConfigPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath error: %v", err)
	}
	expected := filepath.Join(tmp, ".config", "beam", "config.yaml")
	if path != expected {
		t.Errorf("ConfigPath = %q, want %q", path, expected)
	}
}

func TestLoad_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil for missing file")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := DefaultConfig()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil after Save")
	}
}

func TestSaveToAndLoadFrom(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test-config.yaml")

	cfg := DefaultConfig()
	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("SaveTo did not create file")
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadFrom returned nil")
	}
}

func TestLoadFrom_NoFile(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFrom returned nil for missing file")
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0o644)

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
