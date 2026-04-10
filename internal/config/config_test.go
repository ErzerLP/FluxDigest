package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"rss-platform/internal/config"
)

func TestLoadReadsEnvValues(t *testing.T) {
	t.Setenv("APP_HTTP_PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 9090 {
		t.Fatalf("want 9090 got %d", cfg.HTTP.Port)
	}
}

func TestLoadDefaultsPortTo8080(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 8080 {
		t.Fatalf("want 8080 got %d", cfg.HTTP.Port)
	}
}

func TestLoadReadsConfigYAMLAndEnvOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yaml := "http:\n  port: 7000\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tempDir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 7000 {
		t.Fatalf("want 7000 got %d", cfg.HTTP.Port)
	}

	t.Setenv("APP_HTTP_PORT", "9091")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 9091 {
		t.Fatalf("want 9091 got %d", cfg.HTTP.Port)
	}
}
