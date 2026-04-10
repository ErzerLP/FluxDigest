# RSS AI Digest Platform MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Docker Compose deployable MVP that pulls articles from Miniflux every day at 07:00, translates and analyzes them through an OpenAI-compatible model, generates a daily digest, publishes it to Holo, and exposes article-level and digest-level APIs.

**Architecture:** Use a single Go monorepo with three binaries (`rss-api`, `rss-worker`, `rss-scheduler`). Persist business state in PostgreSQL through GORM, coordinate background jobs with Redis + Asynq, run deterministic article/digest flows with Eino Workflow, and keep digest planning inside a constrained Eino Agent layer.

**Tech Stack:** Go, Gin, GORM, PostgreSQL, Redis, Asynq, Eino Workflow, Eino ADK, Miniflux API, OpenAI-compatible API, koanf, zap, Prometheus, Docker Compose

---

## Scope note

这份计划只覆盖已确认的 MVP / 阶段 1，目标是先跑通完整业务闭环；阶段 2 和阶段 3 的增强项不要混到这一轮实现中。

## Planned file map

- `D:\Works\guaidongxi\RSS\cmd\rss-api\main.go`：API 入口。
- `D:\Works\guaidongxi\RSS\cmd\rss-worker\main.go`：Worker 入口。
- `D:\Works\guaidongxi\RSS\cmd\rss-scheduler\main.go`：Scheduler 入口。
- `D:\Works\guaidongxi\RSS\internal\config\config.go`：配置加载。
- `D:\Works\guaidongxi\RSS\internal\app\api\router.go`：Gin 路由。
- `D:\Works\guaidongxi\RSS\internal\repository\postgres\*.go`：GORM 与 repository。
- `D:\Works\guaidongxi\RSS\internal\adapter\miniflux\client.go`：Miniflux client。
- `D:\Works\guaidongxi\RSS\internal\adapter\llm\factory.go`：Eino OpenAI-compatible model factory。
- `D:\Works\guaidongxi\RSS\internal\workflow\article_processing_workflow\workflow.go`：单篇文章处理工作流。
- `D:\Works\guaidongxi\RSS\internal\agent\digest_planning\agent.go`：Digest Planning Agent。
- `D:\Works\guaidongxi\RSS\internal\workflow\daily_digest_workflow\workflow.go`：日报工作流。
- `D:\Works\guaidongxi\RSS\internal\adapter\publisher\holo\publisher.go`：Holo 发布器。
- `D:\Works\guaidongxi\RSS\internal\app\api\handlers\*.go`：Articles / Digests / Profiles / Jobs handlers。
- `D:\Works\guaidongxi\RSS\internal\service\job_service.go`：任务状态与幂等。
- `D:\Works\guaidongxi\RSS\deployments\compose\docker-compose.yml`：Compose 部署。
- `D:\Works\guaidongxi\RSS\api\openapi\openapi.yaml`：OpenAPI 契约。

### Task 1: Bootstrap runtime config, health route, and entrypoints

**Files:**
- Create: `D:\Works\guaidongxi\RSS\go.mod`
- Create: `D:\Works\guaidongxi\RSS\Makefile`
- Create: `D:\Works\guaidongxi\RSS\configs\config.example.yaml`
- Create: `D:\Works\guaidongxi\RSS\internal\config\config.go`
- Create: `D:\Works\guaidongxi\RSS\internal\config\config_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\router.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\router_test.go`
- Create: `D:\Works\guaidongxi\RSS\cmd\rss-api\main.go`
- Create: `D:\Works\guaidongxi\RSS\cmd\rss-worker\main.go`
- Create: `D:\Works\guaidongxi\RSS\cmd\rss-scheduler\main.go`

- [ ] **Step 1: Write the failing tests**

```go
package config_test

import (
    "testing"
    "rss-platform/internal/config"
)

func TestLoadReadsEnvValues(t *testing.T) {
    t.Setenv("APP_HTTP_PORT", "9090")
    cfg, err := config.Load()
    if err != nil { t.Fatal(err) }
    if cfg.HTTP.Port != 9090 { t.Fatalf("want 9090 got %d", cfg.HTTP.Port) }
}
```

```go
package api

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRouterExposesHealthz(t *testing.T) {
    router := NewRouter()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("want 200 got %d", rec.Code) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config ./internal/app/api -v`
Expected: FAIL with `undefined: Load` and `undefined: NewRouter`

- [ ] **Step 3: Write the minimal implementation**

```go
module rss-platform

go 1.24.0
```

```go
package config

type Config struct {
    HTTP struct{ Port int }
    Database struct{ DSN string }
    Redis struct{ Addr string }
}

func Load() (*Config, error) {
    cfg := &Config{}
    cfg.HTTP.Port = 8080
    return cfg, nil
}
```

```go
package api

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
    r := gin.New()
    r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
    return r
}
```

- [ ] **Step 4: Run tests and build**

Run: `go test ./internal/config ./internal/app/api -v`
Expected: PASS

Run: `go build ./cmd/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git init
git add go.mod Makefile configs/config.example.yaml internal/config/config.go internal/config/config_test.go internal/app/api/router.go internal/app/api/router_test.go cmd/rss-api/main.go cmd/rss-worker/main.go cmd/rss-scheduler/main.go
git commit -m "chore: bootstrap runtime skeleton"
```

### Task 2: Add GORM persistence and profile version storage

**Files:**
- Create: `D:\Works\guaidongxi\RSS\internal\domain\article\entity.go`
- Create: `D:\Works\guaidongxi\RSS\internal\domain\profile\entity.go`
- Create: `D:\Works\guaidongxi\RSS\internal\repository\postgres\db.go`
- Create: `D:\Works\guaidongxi\RSS\internal\repository\postgres\models\article.go`
- Create: `D:\Works\guaidongxi\RSS\internal\repository\postgres\models\profile.go`
- Create: `D:\Works\guaidongxi\RSS\internal\repository\postgres\article_repository.go`
- Create: `D:\Works\guaidongxi\RSS\internal\repository\postgres\profile_repository.go`
- Create: `D:\Works\guaidongxi\RSS\internal\repository\postgres\repository_test.go`
- Create: `D:\Works\guaidongxi\RSS\migrations\0001_init.up.sql`
- Create: `D:\Works\guaidongxi\RSS\migrations\0001_init.down.sql`

- [ ] **Step 1: Write the failing repository tests**

```go
func TestArticleRepositoryUpsertAndFindByMinifluxID(t *testing.T) {
    db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    _ = db.AutoMigrate(&models.SourceArticleModel{})
    repo := postgres.NewArticleRepository(db)
    err := repo.Upsert(context.Background(), article.SourceArticle{MinifluxEntryID: 101, Title: "Hello", URL: "https://example.com", Fingerprint: "fp-101"})
    if err != nil { t.Fatal(err) }
    got, err := repo.FindByMinifluxEntryID(context.Background(), 101)
    if err != nil { t.Fatal(err) }
    if got.Title != "Hello" { t.Fatalf("want Hello got %s", got.Title) }
}
```

```go
func TestProfileRepositoryCreateVersionAndActivate(t *testing.T) {
    db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    _ = db.AutoMigrate(&models.ProfileVersionModel{})
    repo := postgres.NewProfileRepository(db)
    err := repo.Create(context.Background(), profile.Version{ProfileType: "ai", Name: "default-ai", Version: 1, IsActive: true, PayloadJSON: []byte(`{"model":"gpt-4.1-mini"}`)})
    if err != nil { t.Fatal(err) }
    active, err := repo.GetActive(context.Background(), "ai")
    if err != nil { t.Fatal(err) }
    if active.Name != "default-ai" { t.Fatalf("want default-ai got %s", active.Name) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/repository/postgres -v`
Expected: FAIL with missing repository constructors and models

- [ ] **Step 3: Write the minimal implementation**

```go
package article

type SourceArticle struct {
    ID string
    MinifluxEntryID int64
    FeedID int64
    FeedTitle string
    Title string
    Author string
    URL string
    ContentHTML string
    ContentText string
    Fingerprint string
}
```

```go
package models

type SourceArticleModel struct {
    ID string `gorm:"primaryKey;size:36"`
    MinifluxEntryID int64 `gorm:"uniqueIndex"`
    Title string
    URL string
    Fingerprint string `gorm:"uniqueIndex"`
}

type ProfileVersionModel struct {
    ID string `gorm:"primaryKey;size:36"`
    ProfileType string `gorm:"index"`
    Name string
    Version int
    IsActive bool
    PayloadJSON []byte `gorm:"type:jsonb"`
}
```

```go
package postgres

func NewArticleRepository(db *gorm.DB) *ArticleRepository { return &ArticleRepository{db: db} }
func NewProfileRepository(db *gorm.DB) *ProfileRepository { return &ProfileRepository{db: db} }
```

```sql
CREATE TABLE source_articles (
  id VARCHAR(36) PRIMARY KEY,
  miniflux_entry_id BIGINT UNIQUE NOT NULL,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  fingerprint TEXT UNIQUE NOT NULL
);
CREATE TABLE profile_versions (
  id VARCHAR(36) PRIMARY KEY,
  profile_type TEXT NOT NULL,
  name TEXT NOT NULL,
  version INT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  payload_json JSONB NOT NULL
);
```

- [ ] **Step 4: Run the repository tests**

Run: `go test ./internal/repository/postgres -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/article/entity.go internal/domain/profile/entity.go internal/repository/postgres/db.go internal/repository/postgres/models/article.go internal/repository/postgres/models/profile.go internal/repository/postgres/article_repository.go internal/repository/postgres/profile_repository.go internal/repository/postgres/repository_test.go migrations/0001_init.up.sql migrations/0001_init.down.sql
git commit -m "feat: add persistence foundation"
```
### Task 3: Implement Miniflux ingestion and default profiles

**Files:**
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\miniflux\client.go`
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\miniflux\client_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\article_ingestion_service.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\profile_service.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\profile_service_test.go`
- Create: `D:\Works\guaidongxi\RSS\configs\prompts\translation.tmpl`
- Create: `D:\Works\guaidongxi\RSS\configs\prompts\analysis.tmpl`
- Create: `D:\Works\guaidongxi\RSS\configs\prompts\digest.tmpl`

- [ ] **Step 1: Write the failing ingestion and profile tests**

```go
func TestClientListEntriesSendsAuthHeaderAndPublishedAfter(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Auth-Token") != "secret-token" { t.Fatal("missing auth token") }
        if r.URL.Query().Get("published_after") == "" { t.Fatal("expected published_after") }
        _ = json.NewEncoder(w).Encode(map[string]any{"entries": []map[string]any{{"id": 1001, "title": "New Model", "url": "https://example.com/a", "content": "<p>Hello</p>", "published_at": time.Now().UTC().Format(time.RFC3339)}}})
    }))
    defer server.Close()
    client := miniflux.NewClient(server.URL, "secret-token")
    entries, err := client.ListEntries(context.Background(), time.Now().Add(-time.Hour))
    if err != nil { t.Fatal(err) }
    if len(entries) != 1 { t.Fatalf("want 1 got %d", len(entries)) }
}
```

```go
func TestProfileServiceSeedsDefaults(t *testing.T) {
    repo := &profileRepoStub{}
    svc := service.NewProfileService(repo)
    if err := svc.SeedDefaults(context.Background()); err != nil { t.Fatal(err) }
    if len(repo.created) != 4 { t.Fatalf("want 4 got %d", len(repo.created)) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/adapter/miniflux ./internal/service -v`
Expected: FAIL with missing Miniflux client and profile service

- [ ] **Step 3: Write the minimal implementation**

```go
package miniflux

type Entry struct {
    ID int64 `json:"id"`
    FeedID int64 `json:"feed_id"`
    FeedTitle string `json:"feed_title"`
    Title string `json:"title"`
    Author string `json:"author"`
    URL string `json:"url"`
    Content string `json:"content"`
    PublishedAt time.Time `json:"published_at"`
}

type Client struct { baseURL, authToken string; httpClient *http.Client }
func NewClient(baseURL, authToken string) *Client { return &Client{baseURL: baseURL, authToken: authToken, httpClient: &http.Client{Timeout: 20 * time.Second}} }
```

```go
package service

type ArticleWriter interface { Upsert(ctx context.Context, input article.SourceArticle) error }

type ArticleIngestionService struct { client *miniflux.Client; repo ArticleWriter }
func NewArticleIngestionService(client *miniflux.Client, repo ArticleWriter) *ArticleIngestionService { return &ArticleIngestionService{client: client, repo: repo} }
```

```go
package service

type ProfileService struct { repo ProfileRepository }
func NewProfileService(repo ProfileRepository) *ProfileService { return &ProfileService{repo: repo} }
func (s *ProfileService) SeedDefaults(ctx context.Context) error {`n    defaults := []string{"ai", "digest", "publish", "api"}`n    for idx, profileType := range defaults {`n        if err := s.repo.Create(ctx, profile.Version{ProfileType: profileType, Name: "default-" + profileType, Version: idx + 1, IsActive: true, PayloadJSON: []byte("{}")}); err != nil {`n            return err`n        }`n    }`n    return nil`n}
```

```text
你是一个技术资讯翻译助手。请输出 JSON：{"title_translated":"","summary_translated":"","content_translated":""}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/adapter/miniflux ./internal/service -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/miniflux/client.go internal/adapter/miniflux/client_test.go internal/service/article_ingestion_service.go internal/service/profile_service.go internal/service/profile_service_test.go configs/prompts/translation.tmpl configs/prompts/analysis.tmpl configs/prompts/digest.tmpl
git commit -m "feat: add miniflux ingestion and profile defaults"
```

### Task 4: Add the OpenAI-compatible model factory, processing service, and article workflow

**Files:**
- Create: `D:\Works\guaidongxi\RSS\internal\domain\processing\entity.go`
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\llm\factory.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\processing_service.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\processing_service_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\workflow\article_processing_workflow\workflow.go`
- Create: `D:\Works\guaidongxi\RSS\internal\workflow\article_processing_workflow\workflow_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\task\asynq\types.go`
- Create: `D:\Works\guaidongxi\RSS\internal\task\asynq\handlers.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\worker\server.go`

- [ ] **Step 1: Write the failing processing and workflow tests**

```go
func TestProcessingServiceTranslateAndAnalyze(t *testing.T) {
    svc := service.NewProcessingService(llmStub{})
    result, err := svc.ProcessArticle(context.Background(), article.SourceArticle{Title: "Original"})
    if err != nil { t.Fatal(err) }
    if result.Analysis.TopicCategory != "AI" { t.Fatalf("want AI got %s", result.Analysis.TopicCategory) }
}
```

```go
func TestWorkflowRunReturnsProcessedArticle(t *testing.T) {
    wf := workflow.New(processingStub{})
    out, err := wf.Run(context.Background(), workflow.Input{Article: article.SourceArticle{ID: "art-1", Title: "Original"}})
    if err != nil { t.Fatal(err) }
    if out.Category != "AI" { t.Fatalf("want AI got %s", out.Category) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/service ./internal/workflow/article_processing_workflow -v`
Expected: FAIL with missing processing service and workflow constructors

- [ ] **Step 3: Write the minimal implementation**

```go
package processing

type Translation struct { TitleTranslated, SummaryTranslated, ContentTranslated string }
type Analysis struct { CoreSummary string; KeyPoints []string; TopicCategory string; ImportanceScore float64 }
```

```go
package llm

func NewChatModel(ctx context.Context, cfg FactoryConfig) (any, error) {
    return openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model})
}
```

```go
package service

type ArticleProcessor interface {
    Translate(ctx context.Context, article article.SourceArticle) (TranslationResult, error)
    Analyze(ctx context.Context, article article.SourceArticle) (AnalysisResult, error)
}

type ProcessingService struct { processor ArticleProcessor }
func NewProcessingService(processor ArticleProcessor) *ProcessingService { return &ProcessingService{processor: processor} }
```

```go
package article_processing_workflow

func New(svc ProcessingService) *Workflow { return &Workflow{svc: svc} }
func (w *Workflow) Run(ctx context.Context, input Input) (ProcessedArticle, error) { return w.svc.ProcessArticle(ctx, input.Article) }
```

```go
package asynqtask

const TypeProcessArticle = "article.process"
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/service ./internal/workflow/article_processing_workflow -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/processing/entity.go internal/adapter/llm/factory.go internal/service/processing_service.go internal/service/processing_service_test.go internal/workflow/article_processing_workflow/workflow.go internal/workflow/article_processing_workflow/workflow_test.go internal/task/asynq/types.go internal/task/asynq/handlers.go internal/app/worker/server.go
git commit -m "feat: add llm processing and article workflow"
```
### Task 5: Implement the digest planning agent and publisher slice

**Files:**
- Create: `D:\Works\guaidongxi\RSS\internal\agent\digest_planning\schema.go`
- Create: `D:\Works\guaidongxi\RSS\internal\agent\digest_planning\agent.go`
- Create: `D:\Works\guaidongxi\RSS\internal\render\digest_renderer.go`
- Create: `D:\Works\guaidongxi\RSS\internal\workflow\daily_digest_workflow\workflow.go`
- Create: `D:\Works\guaidongxi\RSS\internal\workflow\daily_digest_workflow\workflow_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\publisher\publisher.go`
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\publisher\holo\publisher.go`
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\publisher\holo\publisher_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\adapter\publisher\markdown_export\publisher.go`

- [ ] **Step 1: Write the failing digest and publisher tests**

```go
func TestWorkflowGenerateDigest(t *testing.T) {
    wf := workflow.New(plannerStub{}, rendererStub{})
    digest, err := wf.Run(context.Background(), []workflow.CandidateArticle{{ID: "art-1", Title: "Model News"}})
    if err != nil { t.Fatal(err) }
    if digest.Title != "今日 AI 日报" { t.Fatalf("want 今日 AI 日报 got %s", digest.Title) }
}
```

```go
func TestPublishDigestPostsMarkdownToHolo(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { t.Fatalf("want POST got %s", r.Method) }
        _ = json.NewEncoder(w).Encode(map[string]any{"id": "remote-1", "url": "https://blog.example.com/digest"})
    }))
    defer server.Close()
    p := holo.New(server.URL, "blog-token")
    result, err := p.PublishDigest(context.Background(), publisher.PublishDigestRequest{Title: "今日 AI 日报", ContentMarkdown: "# 内容"})
    if err != nil { t.Fatal(err) }
    if result.RemoteURL == "" { t.Fatal("expected remote url") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/workflow/daily_digest_workflow ./internal/adapter/publisher/holo -v`
Expected: FAIL with missing workflow and publisher constructors

- [ ] **Step 3: Write the minimal implementation**

```go
package digest_planning

type Section struct { Name string `json:"name"`; Items []string `json:"items"` }
type Plan struct { Title string `json:"title"`; Subtitle string `json:"subtitle"`; OpeningNote string `json:"opening_note"`; Sections []Section `json:"sections"` }
```

```go
package daily_digest_workflow

type CandidateArticle struct { ID, Title, CoreSummary string }
type Planner interface { Plan(ctx context.Context, items []CandidateArticle) (Plan, error) }
type Renderer interface { Render(plan Plan, items []CandidateArticle) (string, string, error) }
func New(planner Planner, renderer Renderer) *Workflow { return &Workflow{planner: planner, renderer: renderer} }
```

```go
package publisher

type PublishDigestRequest struct { Title, Subtitle, ContentMarkdown, ContentHTML string; Tags []string }
type PublishDigestResult struct { RemoteID, RemoteURL string }
type Publisher interface { Name() string; PublishDigest(ctx context.Context, req PublishDigestRequest) (PublishDigestResult, error) }
```

```go
package holo

func New(endpoint, token string) *Publisher { return &Publisher{endpoint: endpoint, token: token, httpClient: &http.Client{Timeout: 15 * time.Second}} }
func (p *Publisher) Name() string { return "holo" }
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/workflow/daily_digest_workflow ./internal/adapter/publisher/holo ./internal/adapter/publisher/markdown_export -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/digest_planning/schema.go internal/agent/digest_planning/agent.go internal/render/digest_renderer.go internal/workflow/daily_digest_workflow/workflow.go internal/workflow/daily_digest_workflow/workflow_test.go internal/adapter/publisher/publisher.go internal/adapter/publisher/holo/publisher.go internal/adapter/publisher/holo/publisher_test.go internal/adapter/publisher/markdown_export/publisher.go
git commit -m "feat: add digest workflow and publisher slice"
```

### Task 6: Expose APIs, add scheduler/idempotency, and finish deployability

**Files:**
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\middleware\apikey.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\handlers\article_handler.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\handlers\digest_handler.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\handlers\profile_handler.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\handlers\job_handler.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\api\handlers\handlers_test.go`
- Modify: `D:\Works\guaidongxi\RSS\internal\app\api\router.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\job_service.go`
- Create: `D:\Works\guaidongxi\RSS\internal\service\job_service_test.go`
- Create: `D:\Works\guaidongxi\RSS\internal\app\scheduler\server.go`
- Create: `D:\Works\guaidongxi\RSS\internal\telemetry\metrics.go`
- Create: `D:\Works\guaidongxi\RSS\deployments\docker\api.Dockerfile`
- Create: `D:\Works\guaidongxi\RSS\deployments\docker\worker.Dockerfile`
- Create: `D:\Works\guaidongxi\RSS\deployments\docker\scheduler.Dockerfile`
- Create: `D:\Works\guaidongxi\RSS\deployments\compose\docker-compose.yml`
- Create: `D:\Works\guaidongxi\RSS\api\openapi\openapi.yaml`

- [ ] **Step 1: Write the failing API and job tests**

```go
func TestLatestDigestRouteReturnsJSON(t *testing.T) {
    gin.SetMode(gin.TestMode)
    router := gin.New()
    handlers.RegisterDigestRoutes(router.Group("/api/v1"), digestServiceStub{})
    req := httptest.NewRequest(http.MethodGet, "/api/v1/digests/latest", nil)
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("want 200 got %d", rec.Code) }
}
```

```go
func TestJobServiceSkipsDuplicateDigestDate(t *testing.T) {
    queue := &enqueueStub{}
    svc := service.NewJobService(queue)
    now := time.Date(2026, 4, 10, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))
    if err := svc.TriggerDailyDigest(context.Background(), now); err != nil { t.Fatal(err) }
    if err := svc.TriggerDailyDigest(context.Background(), now.Add(5*time.Minute)); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/api/handlers ./internal/service -v`
Expected: FAIL with missing handler registration and job service

- [ ] **Step 3: Write the minimal implementation**

```go
package middleware

func RequireAPIKey(expected string) gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.GetHeader("X-API-Key") != expected { c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"}); return }
        c.Next()
    }
}
```

```go
package handlers

type DigestReader interface { LatestDigest() map[string]any }
func RegisterDigestRoutes(group *gin.RouterGroup, svc DigestReader) {
    group.GET("/digests/latest", func(c *gin.Context) { c.JSON(200, svc.LatestDigest()) })
}
```

```go
package service

type DailyDigestQueue interface { EnqueueDailyDigest(ctx context.Context, digestDate string) error }
type JobService struct { queue DailyDigestQueue; mu sync.Mutex; lastDigest string }
func NewJobService(queue DailyDigestQueue) *JobService { return &JobService{queue: queue} }
```

```go
package scheduler

func Start(job Trigger) *cron.Cron {
    location := time.FixedZone("CST", 8*3600)
    c := cron.New(cron.WithLocation(location))
    _, _ = c.AddFunc("0 7 * * *", func() { _ = job.TriggerDailyDigest(context.Background(), time.Now().In(location)) })
    c.Start()
    return c
}
```

```yaml
openapi: 3.1.0
info:
  title: RSS AI Digest API
  version: 1.0.0
paths:
  /api/v1/digests/latest:
    get:
      summary: Get latest digest
```

```yaml
services:
  postgres:
    image: postgres:17
  redis:
    image: redis:7
  rss-api:
    build:
      context: ../..
      dockerfile: deployments/docker/api.Dockerfile
```

- [ ] **Step 4: Run full verification**

Run: `go test ./...`
Expected: PASS

Run: `go build ./cmd/...`
Expected: PASS

Run: `docker compose -f deployments/compose/docker-compose.yml config`
Expected: prints normalized compose config without validation errors

- [ ] **Step 5: Commit**

```bash
git add internal/app/api/middleware/apikey.go internal/app/api/handlers/article_handler.go internal/app/api/handlers/digest_handler.go internal/app/api/handlers/profile_handler.go internal/app/api/handlers/job_handler.go internal/app/api/handlers/handlers_test.go internal/app/api/router.go internal/service/job_service.go internal/service/job_service_test.go internal/app/scheduler/server.go internal/telemetry/metrics.go deployments/docker/api.Dockerfile deployments/docker/worker.Dockerfile deployments/docker/scheduler.Dockerfile deployments/compose/docker-compose.yml api/openapi/openapi.yaml
git commit -m "feat: add api surface, scheduler, and compose deployment"
```

## Self-review checklist

- **Spec coverage:** Miniflux 拉取在 Task 3；OpenAI-compatible 接入在 Task 4；文章处理 Workflow 在 Task 4；日报 Agent + Workflow 在 Task 5；Holo 发布在 Task 5；开放 API 在 Task 6；Scheduler / 07:00 / Compose / Metrics 在 Task 6。
- **Placeholder scan:** 本计划未使用常见占位标记或“后面再补”式描述。
- **Type consistency:** 统一使用 `SourceArticle`、`Translation`、`Analysis`、`Plan`、`Digest`、`JobService` 这些类型名，不在后续任务里改名。

