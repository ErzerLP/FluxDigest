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

func TestLoadReadsAdminSecurityEnvValues(t *testing.T) {
	t.Setenv("APP_ADMIN_SESSION_SECRET", "admin-session-secret")
	t.Setenv("APP_SECRET_KEY", "security-secret-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Admin.SessionSecret != "admin-session-secret" {
		t.Fatalf("want admin-session-secret got %q", cfg.Admin.SessionSecret)
	}
	if cfg.Security.SecretKey != "security-secret-key" {
		t.Fatalf("want security-secret-key got %q", cfg.Security.SecretKey)
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
	if cfg.Job.Queue != "default" {
		t.Fatalf("want default queue got %q", cfg.Job.Queue)
	}
	if cfg.Worker.Concurrency != 10 {
		t.Fatalf("want worker concurrency 10 got %d", cfg.Worker.Concurrency)
	}
	if cfg.LLM.Model != "MiniMax-M2.7" {
		t.Fatalf("want default llm model MiniMax-M2.7 got %q", cfg.LLM.Model)
	}
	if len(cfg.LLM.FallbackModels) != 1 || cfg.LLM.FallbackModels[0] != "mimo-v2-pro" {
		t.Fatalf("want default fallback models [mimo-v2-pro] got %#v", cfg.LLM.FallbackModels)
	}
}

func TestLoadReadsConfigYAMLAndEnvOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yaml := "http:\n  port: 7000\njob:\n  api_key: yaml-secret\n  queue: digest\nworker:\n  concurrency: 3\n"
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
	if cfg.Job.APIKey != "yaml-secret" {
		t.Fatalf("want yaml-secret got %q", cfg.Job.APIKey)
	}
	if cfg.Job.Queue != "digest" {
		t.Fatalf("want digest got %q", cfg.Job.Queue)
	}
	if cfg.Worker.Concurrency != 3 {
		t.Fatalf("want 3 got %d", cfg.Worker.Concurrency)
	}

	t.Setenv("APP_HTTP_PORT", "9091")
	t.Setenv("APP_JOB_API_KEY", "env-secret")
	t.Setenv("APP_JOB_QUEUE", "default")
	t.Setenv("APP_WORKER_CONCURRENCY", "9")
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 9091 {
		t.Fatalf("want 9091 got %d", cfg.HTTP.Port)
	}
	if cfg.Job.APIKey != "env-secret" {
		t.Fatalf("want env-secret got %q", cfg.Job.APIKey)
	}
	if cfg.Job.Queue != "default" {
		t.Fatalf("want default got %q", cfg.Job.Queue)
	}
	if cfg.Worker.Concurrency != 9 {
		t.Fatalf("want 9 got %d", cfg.Worker.Concurrency)
	}
}

func TestLoadReadsMinifluxLLMAndPublishConfig(t *testing.T) {
	t.Setenv("APP_MINIFLUX_BASE_URL", "https://miniflux.local")
	t.Setenv("APP_MINIFLUX_AUTH_TOKEN", "miniflux-token")
	t.Setenv("APP_LLM_BASE_URL", "https://llm.local/v1")
	t.Setenv("APP_LLM_API_KEY", "llm-token")
	t.Setenv("APP_LLM_MODEL", "MiniMax-M2.7")
	t.Setenv("APP_LLM_FALLBACK_MODELS", "mimo-v2-pro, kimi-k2.5 ")
	t.Setenv("APP_LLM_TIMEOUT_MS", "45000")
	t.Setenv("APP_PUBLISH_HALO_BASE_URL", "https://halo.local")
	t.Setenv("APP_PUBLISH_HALO_TOKEN", "halo-token")
	t.Setenv("APP_PUBLISH_CHANNEL", "halo")
	t.Setenv("APP_PUBLISH_OUTPUT_DIR", "data/output")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Miniflux.BaseURL != "https://miniflux.local" {
		t.Fatalf("unexpected base url %q", cfg.Miniflux.BaseURL)
	}
	if cfg.Miniflux.AuthToken != "miniflux-token" {
		t.Fatalf("unexpected auth token %q", cfg.Miniflux.AuthToken)
	}
	if cfg.LLM.BaseURL != "https://llm.local/v1" {
		t.Fatalf("unexpected llm base url %q", cfg.LLM.BaseURL)
	}
	if cfg.LLM.APIKey != "llm-token" {
		t.Fatalf("unexpected llm api key %q", cfg.LLM.APIKey)
	}
	if cfg.LLM.Model != "MiniMax-M2.7" {
		t.Fatalf("unexpected llm model %q", cfg.LLM.Model)
	}
	if len(cfg.LLM.FallbackModels) != 2 || cfg.LLM.FallbackModels[0] != "mimo-v2-pro" || cfg.LLM.FallbackModels[1] != "kimi-k2.5" {
		t.Fatalf("unexpected llm fallback models %#v", cfg.LLM.FallbackModels)
	}
	if cfg.LLM.TimeoutMS != 45000 {
		t.Fatalf("unexpected llm timeout %d", cfg.LLM.TimeoutMS)
	}
	if cfg.Publish.HaloBaseURL != "https://halo.local" {
		t.Fatalf("unexpected halo base url %q", cfg.Publish.HaloBaseURL)
	}
	if cfg.Publish.HaloToken != "halo-token" {
		t.Fatalf("unexpected halo token %q", cfg.Publish.HaloToken)
	}
	if cfg.Publish.Channel != "halo" {
		t.Fatalf("unexpected publish channel %q", cfg.Publish.Channel)
	}
	if cfg.Publish.OutputDir != "data/output" {
		t.Fatalf("unexpected output dir %q", cfg.Publish.OutputDir)
	}
}

func TestLoadHaloYAMLCanBeOverriddenByEnv(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yaml := "publish:\n  channel: halo\n  halo_base_url: https://yaml-halo.local\n  halo_token: yaml-token\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tempDir)
	t.Setenv("APP_PUBLISH_HALO_BASE_URL", "https://env-halo.local")
	t.Setenv("APP_PUBLISH_HALO_TOKEN", "env-token")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Publish.Channel != "halo" {
		t.Fatalf("want halo channel got %q", cfg.Publish.Channel)
	}
	if cfg.Publish.HaloBaseURL != "https://env-halo.local" {
		t.Fatalf("want env halo base url got %q", cfg.Publish.HaloBaseURL)
	}
	if cfg.Publish.HaloToken != "env-token" {
		t.Fatalf("want env halo token got %q", cfg.Publish.HaloToken)
	}
}

func TestLoadAdminSecurityYAMLCanBeOverriddenByEnv(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yaml := "admin:\n  session_secret: yaml-admin-secret\nsecurity:\n  secret_key: yaml-security-key\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tempDir)
	t.Setenv("APP_ADMIN_SESSION_SECRET", "env-admin-secret")
	t.Setenv("APP_SECRET_KEY", "env-security-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Admin.SessionSecret != "env-admin-secret" {
		t.Fatalf("want env-admin-secret got %q", cfg.Admin.SessionSecret)
	}
	if cfg.Security.SecretKey != "env-security-key" {
		t.Fatalf("want env-security-key got %q", cfg.Security.SecretKey)
	}
}
