package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/hibiken/asynq"

	llmadapter "rss-platform/internal/adapter/llm"
	"rss-platform/internal/config"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/service"
	"rss-platform/internal/telemetry"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type hangingChatModelStub struct{}

func (hangingChatModelStub) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingChatModelStub) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not implemented in test stub")
}

type queueStub struct {
	dates             []string
	dailyDigestForces []bool
	articleIDs        []string
	articleForces     []bool
}

func (s *queueStub) EnqueueDailyDigest(_ context.Context, digestDate string) error {
	s.dates = append(s.dates, digestDate)
	return nil
}

func (s *queueStub) EnqueueDailyDigestWithOptions(_ context.Context, digestDate string, opts service.DailyDigestTriggerOptions) error {
	s.dates = append(s.dates, digestDate)
	s.dailyDigestForces = append(s.dailyDigestForces, opts.Force)
	return nil
}

func (s *queueStub) EnqueueArticleReprocess(_ context.Context, articleID string, force bool) error {
	s.articleIDs = append(s.articleIDs, articleID)
	s.articleForces = append(s.articleForces, force)
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

	_, _, err := buildAPIRouter(context.Background(), cfg, &queueStub{}, &queueStub{}, func(context.Context, string) (*gorm.DB, dbCloser, error) {
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

	router, closer, err := buildAPIRouter(context.Background(), cfg, queue, queue, func(_ context.Context, dsn string) (*gorm.DB, dbCloser, error) {
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

func TestBuildAPIRouterDailyDigestForceUsesQueueOptions(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.DSN = "postgres://rss:rss@postgres:5432/rss?sslmode=disable"
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	queue := &queueStub{}
	db := newAPITestDB(t)
	router, closer, err := buildAPIRouter(context.Background(), cfg, queue, queue, func(context.Context, string) (*gorm.DB, dbCloser, error) {
		return db, closeStub{}, nil
	}, telemetry.NewMetrics())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/daily-digest", bytes.NewBufferString(`{"trigger_at":"2026-04-10T07:00:00+08:00","force":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202 got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(queue.dailyDigestForces) != 1 || !queue.dailyDigestForces[0] {
		t.Fatalf("want force=true got %#v", queue.dailyDigestForces)
	}
}

func TestBuildAPIRouterArticleReprocessEnqueuesTask(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.DSN = "postgres://rss:rss@postgres:5432/rss?sslmode=disable"
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	queue := &queueStub{}
	db := newAPITestDB(t)
	router, closer, err := buildAPIRouter(context.Background(), cfg, queue, queue, func(context.Context, string) (*gorm.DB, dbCloser, error) {
		return db, closeStub{}, nil
	}, telemetry.NewMetrics())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer.Close() }()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/article-reprocess", bytes.NewBufferString(`{"article_id":"art-1","force":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202 got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(queue.articleIDs) != 1 || queue.articleIDs[0] != "art-1" {
		t.Fatalf("want article id art-1 got %#v", queue.articleIDs)
	}
	if len(queue.articleForces) != 1 || !queue.articleForces[0] {
		t.Fatalf("want force=true got %#v", queue.articleForces)
	}
}

func TestMapDailyDigestEnqueueErrorMapsTaskIDConflictWhenNotForce(t *testing.T) {
	got := mapDailyDigestEnqueueError(asynq.ErrTaskIDConflict, false)
	if !errors.Is(got, service.ErrDailyDigestAlreadyQueued) {
		t.Fatalf("want ErrDailyDigestAlreadyQueued got %v", got)
	}
}

func TestMapDailyDigestEnqueueErrorKeepsConflictWhenForce(t *testing.T) {
	got := mapDailyDigestEnqueueError(asynq.ErrTaskIDConflict, true)
	if !errors.Is(got, asynq.ErrTaskIDConflict) {
		t.Fatalf("want asynq.ErrTaskIDConflict got %v", got)
	}
	if errors.Is(got, service.ErrDailyDigestAlreadyQueued) {
		t.Fatalf("force enqueue should not map to ErrDailyDigestAlreadyQueued, got %v", got)
	}
}

func TestMapArticleReprocessEnqueueErrorMapsTaskIDConflictWhenNotForce(t *testing.T) {
	got := mapArticleReprocessEnqueueError(asynq.ErrTaskIDConflict, false)
	if !errors.Is(got, service.ErrArticleReprocessAlreadyQueued) {
		t.Fatalf("want ErrArticleReprocessAlreadyQueued got %v", got)
	}
}

func TestMapArticleReprocessEnqueueErrorKeepsConflictWhenForce(t *testing.T) {
	got := mapArticleReprocessEnqueueError(asynq.ErrTaskIDConflict, true)
	if !errors.Is(got, asynq.ErrTaskIDConflict) {
		t.Fatalf("want asynq.ErrTaskIDConflict got %v", got)
	}
	if errors.Is(got, service.ErrArticleReprocessAlreadyQueued) {
		t.Fatalf("force enqueue should not map to ErrArticleReprocessAlreadyQueued, got %v", got)
	}
}

func TestBuildAPIRouterExposesRuntimeDataFromDatabase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.DSN = "postgres://rss:rss@postgres:5432/rss?sslmode=disable"
	cfg.Job.APIKey = "secret"
	cfg.Job.Queue = "default"

	db := newAPITestDB(t)
	router, closer, err := buildAPIRouter(context.Background(), cfg, &queueStub{}, &queueStub{}, func(context.Context, string) (*gorm.DB, dbCloser, error) {
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
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO article_dossiers (
			id, article_id, processing_id, digest_date, version, is_active,
			title_translated, summary_polished, core_summary, key_points_json,
			topic_category, importance_score, recommendation_reason, reading_value,
			priority_level, content_polished_markdown, analysis_longform_markdown,
			background_context, impact_analysis, debate_points_json, target_audience,
			publish_suggestion, suggestion_reason, suggested_channels_json, suggested_tags_json,
			suggested_categories_json, translation_prompt_version, analysis_prompt_version,
			dossier_prompt_version, llm_profile_version, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dos-1",
		"art-1",
		"proc-1",
		time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		1,
		true,
		"模型新闻",
		"润色摘要",
		"这是核心观点",
		`["要点一"]`,
		"AI",
		0.8,
		"值得跟进",
		"高",
		"high",
		"## 正文",
		"## 分析",
		"",
		"",
		`[]`,
		"",
		"suggested",
		"",
		`["holo"]`,
		`["ai"]`,
		`["tech"]`,
		6,
		6,
		6,
		4,
		time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
	).Error; err != nil {
		t.Fatalf("create dossier: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO article_publish_states (
			id, dossier_id, state, approved_by, decision_note, publish_channel,
			remote_id, remote_url, error_message, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"pub-1",
		"dos-1",
		"suggested",
		"",
		"",
		"holo",
		"",
		"https://example.com/posts/1",
		"",
		time.Date(2026, 4, 11, 8, 30, 0, 0, time.UTC),
	).Error; err != nil {
		t.Fatalf("create publish state: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO daily_digest_items (
			id, digest_id, dossier_id, section_name, importance_bucket, position, is_featured, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"item-1",
		"digest-1",
		"dos-1",
		"重点速览",
		"featured",
		1,
		true,
		time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
	).Error; err != nil {
		t.Fatalf("create digest item: %v", err)
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
	items, ok := digestBody["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("want digest items got %#v", digestBody["items"])
	}

	dossiersReq := httptest.NewRequest(http.MethodGet, "/api/v1/dossiers", nil)
	dossiersRec := httptest.NewRecorder()
	router.ServeHTTP(dossiersRec, dossiersReq)
	if dossiersRec.Code != http.StatusOK {
		t.Fatalf("want dossiers 200 got %d body=%s", dossiersRec.Code, dossiersRec.Body.String())
	}

	var dossiersBody struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(dossiersRec.Body.Bytes(), &dossiersBody); err != nil {
		t.Fatalf("unmarshal dossiers: %v", err)
	}
	if len(dossiersBody.Items) != 1 {
		t.Fatalf("want 1 dossier got %d", len(dossiersBody.Items))
	}
	if dossiersBody.Items[0]["title_translated"] != "模型新闻" {
		t.Fatalf("want dossier title 模型新闻 got %#v", dossiersBody.Items[0]["title_translated"])
	}

	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/llm/active", nil)
	profileRec := httptest.NewRecorder()
	router.ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("want profile 200 got %d body=%s", profileRec.Code, profileRec.Body.String())
	}

	var profileBody map[string]any
	if err := json.Unmarshal(profileRec.Body.Bytes(), &profileBody); err != nil {
		t.Fatalf("unmarshal profile: %v", err)
	}
	if profileBody["name"] != "default-llm" {
		t.Fatalf("want default-llm got %#v", profileBody["name"])
	}
}

func TestAdminLLMConnectivityCheckerUsesInternalTimeout(t *testing.T) {
	t.Parallel()

	checker := adminLLMConnectivityChecker{
		timeout: 20 * time.Millisecond,
		newChatModel: func(context.Context, llmadapter.FactoryConfig) (model.BaseChatModel, error) {
			return hangingChatModelStub{}, nil
		},
	}

	parentCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	latency, err := checker.Check(parentCtx, service.LLMTestDraft{
		BaseURL: "https://llm.local/v1",
		Model:   "gpt-4.1-mini",
		APIKey:  "token",
	})
	elapsed := time.Since(startedAt)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout error got %v", err)
	}
	if elapsed >= 120*time.Millisecond {
		t.Fatalf("want checker to stop early, elapsed=%s latency=%s", elapsed, latency)
	}
	if latency <= 0 {
		t.Fatalf("want positive latency got %s", latency)
	}
}
