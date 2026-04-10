package config_test

import (
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