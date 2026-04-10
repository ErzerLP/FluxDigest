package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rss-platform/internal/config"
	"rss-platform/internal/telemetry"
)

type queueStub struct {
	dates []string
}

func (s *queueStub) EnqueueDailyDigest(_ context.Context, digestDate string) error {
	s.dates = append(s.dates, digestDate)
	return nil
}

type closeStub struct{}

func (closeStub) Close() error { return nil }

func TestBuildAPIRouterRequiresDatabaseDSN(t *testing.T) {
	cfg := &config.Config{}
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	_, _, err := buildAPIRouter(context.Background(), cfg, &queueStub{}, func(context.Context, string) (dbCloser, error) {
		return closeStub{}, nil
	}, telemetry.NewMetrics())
	if err == nil {
		t.Fatal("want error for missing database dsn")
	}
}

func TestBuildAPIRouterConnectsPostgresAndSharesMetrics(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.DSN = "postgres://rss:rss@postgres:5432/rss?sslmode=disable"
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	queue := &queueStub{}
	metrics := telemetry.NewMetrics()
	called := 0
	gotDSN := ""

	router, closer, err := buildAPIRouter(context.Background(), cfg, queue, func(_ context.Context, dsn string) (dbCloser, error) {
		called++
		gotDSN = dsn
		return closeStub{}, nil
	}, metrics)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	if called != 1 {
		t.Fatalf("want connect called once got %d", called)
	}
	if gotDSN != cfg.Database.DSN {
		t.Fatalf("want dsn %q got %q", cfg.Database.DSN, gotDSN)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/daily-digest", bytes.NewBufferString(`{"trigger_at":"2026-04-10T07:00:00+08:00"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202 got %d body=%s", rec.Code, rec.Body.String())
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	router.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", metricsRec.Code)
	}

	body := metricsRec.Body.String()
	want := fmt.Sprintf("rss_daily_digest_triggered_total %d", 1)
	if !strings.Contains(body, want) {
		t.Fatalf("want metrics body to contain %q got %s", want, body)
	}
	if len(queue.dates) != 1 || queue.dates[0] != "2026-04-10" {
		t.Fatalf("want queued digest date 2026-04-10 got %#v", queue.dates)
	}
}
