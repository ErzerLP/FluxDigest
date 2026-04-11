package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rss-platform/internal/config"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/telemetry"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func newAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s-api?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open api test db: %v", err)
	}

	return db
}

func TestBuildAPIRouterRequiresDatabaseDSN(t *testing.T) {
	cfg := &config.Config{}
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	_, _, err := buildAPIRouter(context.Background(), cfg, &queueStub{}, func(context.Context, string) (*gorm.DB, dbCloser, error) {
		return newAPITestDB(t), closeStub{}, nil
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
	db := newAPITestDB(t)

	router, closer, err := buildAPIRouter(context.Background(), cfg, queue, func(_ context.Context, dsn string) (*gorm.DB, dbCloser, error) {
		called++
		gotDSN = dsn
		return db, closeStub{}, nil
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

func TestBuildAPIRouterExposesRuntimeDataFromDatabase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.DSN = "postgres://rss:rss@postgres:5432/rss?sslmode=disable"
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	db := newAPITestDB(t)
	router, closer, err := buildAPIRouter(context.Background(), cfg, &queueStub{}, func(context.Context, string) (*gorm.DB, dbCloser, error) {
		return db, closeStub{}, nil
	}, telemetry.NewMetrics())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	ctx := context.Background()
	if err := db.WithContext(ctx).Create(&models.SourceArticleModel{
		ID:              "art-1",
		MinifluxEntryID: 101,
		FeedID:          11,
		FeedTitle:       "Tech Feed",
		Title:           "Model News",
		Author:          "Alice",
		URL:             "https://example.com/model-news",
		ContentHTML:     "<p>Hello</p>",
		ContentText:     "Hello",
		Fingerprint:     "fp-art-1",
	}).Error; err != nil {
		t.Fatalf("create source article: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO article_processings (
			id, article_id, title_translated, summary_translated, content_translated,
			core_summary, key_points_json, topic_category, importance_score, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"proc-1",
		"art-1",
		"模型新闻",
		"这是摘要",
		"这是正文",
		"这是核心观点",
		`["要点一"]`,
		"AI",
		0.8,
		time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
	).Error; err != nil {
		t.Fatalf("create processing: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO daily_digests (
			id, digest_date, title, subtitle, content_markdown, content_html,
			remote_id, remote_url, publish_state, publish_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"digest-1",
		time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		"今日 AI 日报",
		"副标题",
		"# 今日 AI 日报",
		"<h1>今日 AI 日报</h1>",
		"remote-1",
		"https://example.com/digest",
		"published",
		"",
		time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
	).Error; err != nil {
		t.Fatalf("create digest: %v", err)
	}

	articlesReq := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	articlesRec := httptest.NewRecorder()
	router.ServeHTTP(articlesRec, articlesReq)
	if articlesRec.Code != http.StatusOK {
		t.Fatalf("want articles 200 got %d body=%s", articlesRec.Code, articlesRec.Body.String())
	}

	var articlesBody struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(articlesRec.Body.Bytes(), &articlesBody); err != nil {
		t.Fatalf("unmarshal articles: %v", err)
	}
	if len(articlesBody.Items) != 1 {
		t.Fatalf("want 1 article got %d", len(articlesBody.Items))
	}
	if articlesBody.Items[0]["title_translated"] != "模型新闻" {
		t.Fatalf("want translated title 模型新闻 got %#v", articlesBody.Items[0]["title_translated"])
	}
	if articlesBody.Items[0]["core_summary"] != "这是核心观点" {
		t.Fatalf("want core summary got %#v", articlesBody.Items[0]["core_summary"])
	}

	digestReq := httptest.NewRequest(http.MethodGet, "/api/v1/digests/latest", nil)
	digestRec := httptest.NewRecorder()
	router.ServeHTTP(digestRec, digestReq)
	if digestRec.Code != http.StatusOK {
		t.Fatalf("want digest 200 got %d body=%s", digestRec.Code, digestRec.Body.String())
	}

	var digestBody map[string]any
	if err := json.Unmarshal(digestRec.Body.Bytes(), &digestBody); err != nil {
		t.Fatalf("unmarshal digest: %v", err)
	}
	if digestBody["title"] != "今日 AI 日报" {
		t.Fatalf("want digest title 今日 AI 日报 got %#v", digestBody["title"])
	}
	if digestBody["publish_state"] != "published" {
		t.Fatalf("want publish_state published got %#v", digestBody["publish_state"])
	}

	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/ai/active", nil)
	profileRec := httptest.NewRecorder()
	router.ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("want profile 200 got %d body=%s", profileRec.Code, profileRec.Body.String())
	}

	var profileBody map[string]any
	if err := json.Unmarshal(profileRec.Body.Bytes(), &profileBody); err != nil {
		t.Fatalf("unmarshal profile: %v", err)
	}
	if profileBody["name"] != "default-ai" {
		t.Fatalf("want default-ai got %#v", profileBody["name"])
	}
}
