# FluxDigest Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a React-based FluxDigest configuration console that can edit runtime config, test integrations, manually trigger daily digests, inspect recent runs, and ship as part of the existing Go deployment.

**Architecture:** Keep editable runtime config in PostgreSQL through versioned `profile_versions` records plus persisted `job_runs` records, expose a new Admin API from `rss-api`, and have `rss-worker` / `rss-scheduler` resolve active config from the database with environment defaults as fallback. Serve the production React SPA from `rss-api`, while local development uses Vite with an API proxy.

**Tech Stack:** Go, Gin, GORM, PostgreSQL, Redis, Asynq, React, Vite, TypeScript, Ant Design, TanStack Query, React Hook Form, Zod, Vitest, React Testing Library

---

## Scope note

这份计划覆盖一个完整闭环：后端 Admin API、运行时配置接入、任务历史、React Web UI 和部署集成。不要在这一轮加入多用户、权限系统、Prompt 历史版本树、可视化流程编排器。

## Planned file map

- `internal/domain/profile/entity.go`：补齐 profile type 常量。
- `internal/service/admin_config_service.go`：配置快照、密钥掩码、保存逻辑。
- `internal/service/runtime_config_service.go`：DB 配置解析，环境变量兜底。
- `internal/repository/postgres/models/job_run.go`：任务/测试记录 model。
- `internal/repository/postgres/job_run_repository.go`：任务/测试记录仓储。
- `internal/service/admin_status_service.go`：Dashboard 状态聚合。
- `internal/service/admin_test_service.go`：LLM / Miniflux / Publish 测试逻辑。
- `internal/service/job_run_query_service.go`：Jobs 查询逻辑。
- `internal/app/api/handlers/admin_handler.go`：Admin API handler。
- `internal/app/api/router.go`：注册 admin 路由与 SPA fallback。
- `cmd/rss-api/main.go`：注入 admin services。
- `cmd/rss-worker/main.go`：按 DB 配置构造 runtime，记录 run 状态。
- `cmd/rss-scheduler/main.go`：按 DB scheduler 配置触发。
- `internal/app/scheduler/server.go`：改成轮询调度配置。
- `migrations/0004_job_runs.up.sql` / `0004_job_runs.down.sql`：新增 `job_runs` 表。
- `api/openapi/openapi.yaml`：补充 admin 契约。
- `web/package.json`、`web/vite.config.ts`、`web/src/**`：React Web UI。
- `deployments/docker/api.Dockerfile`：构建并打包前端静态资源。
- `scripts/smoke-compose.ps1`：增加 Admin API 与 SPA 验证。

### Task 1: Add profile-backed admin config service

**Files:**
- Modify: `internal/domain/profile/entity.go`
- Modify: `internal/service/profile_service.go`
- Modify: `internal/service/profile_service_test.go`
- Create: `internal/service/admin_config_service.go`
- Create: `internal/service/admin_config_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestAdminConfigServiceSnapshotMasksSecrets(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {ProfileType: profile.TypeLLM, Version: 2, IsActive: true, PayloadJSON: []byte(`{"base_url":"https://llm.local/v1","model":"gpt-4.1-mini","api_key":"secret-llm","is_enabled":true}`)},
	}}
	svc := service.NewAdminConfigService(repo)

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil { t.Fatal(err) }
	if snapshot.LLM.APIKey.MaskedValue != "secr****" { t.Fatalf("want secr**** got %q", snapshot.LLM.APIKey.MaskedValue) }
}
```

```go
func TestAdminConfigServiceUpdateLLMKeepAndClearSecret(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {ProfileType: profile.TypeLLM, Version: 2, IsActive: true, PayloadJSON: []byte(`{"api_key":"secret-llm"}`)},
	}}
	svc := service.NewAdminConfigService(repo)

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{BaseURL: "https://proxy.local/v1", Model: "gpt-4.1", APIKey: service.SecretInput{Mode: service.SecretModeKeep}}); err != nil { t.Fatal(err) }
	if !strings.Contains(string(repo.created[0].PayloadJSON), `"api_key":"secret-llm"`) { t.Fatalf("expected kept secret payload got %s", repo.created[0].PayloadJSON) }

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{BaseURL: "https://proxy.local/v1", Model: "gpt-4.1", APIKey: service.SecretInput{Mode: service.SecretModeClear}}); err != nil { t.Fatal(err) }
	if strings.Contains(string(repo.created[1].PayloadJSON), `"api_key":"secret-llm"`) { t.Fatalf("expected cleared secret payload got %s", repo.created[1].PayloadJSON) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service -run 'TestAdminConfigService|TestProfileService' -v`
Expected: FAIL with missing `NewAdminConfigService`, `profile.TypeLLM`, and secret DTOs.

- [ ] **Step 3: Write minimal implementation**

```go
package profile

const (
	TypeLLM       = "llm"
	TypeMiniflux  = "miniflux"
	TypePrompts   = "prompts"
	TypePublish   = "publish"
	TypeScheduler = "scheduler"
)
```

```go
package service

type SecretMode string

const (
	SecretModeKeep    SecretMode = "keep"
	SecretModeReplace SecretMode = "replace"
	SecretModeClear   SecretMode = "clear"
)

type SecretInput struct { Mode SecretMode `json:"mode"`; Value string `json:"value,omitempty"` }
type SecretView struct { IsSet bool `json:"is_set"`; MaskedValue string `json:"masked_value,omitempty"` }

func maskSecret(value string) SecretView {
	if value == "" { return SecretView{} }
	if len(value) <= 4 { return SecretView{IsSet: true, MaskedValue: "****"} }
	return SecretView{IsSet: true, MaskedValue: value[:4] + "****"}
}

func applySecret(payload map[string]any, current map[string]any, key string, input SecretInput) {
	switch input.Mode {
	case SecretModeReplace:
		if input.Value != "" { payload[key] = input.Value }
	case SecretModeClear:
		payload[key] = ""
	default:
		if existing, ok := current[key].(string); ok { payload[key] = existing }
	}
}
```

```go
func (s *AdminConfigService) GetSnapshot(ctx context.Context) (AdminConfigSnapshot, error) {
	payload, version, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil { return AdminConfigSnapshot{}, err }
	return AdminConfigSnapshot{LLM: LLMConfigView{BaseURL: stringValue(payload, "base_url"), Model: stringValue(payload, "model"), APIKey: maskSecret(stringValue(payload, "api_key")), Enabled: boolValue(payload, "is_enabled", true), TimeoutMS: intValue(payload, "timeout_ms", 30000), Version: version}}, nil
}
```

```go
func (s *ProfileService) SeedDefaults(ctx context.Context) error {
	defaults := []struct{ profileType, name string; payload map[string]any }{
		{profileType: profile.TypeLLM, name: "default-llm", payload: map[string]any{"base_url": "", "model": "gpt-4.1-mini", "timeout_ms": 30000, "is_enabled": true}},
		{profileType: profile.TypeMiniflux, name: "default-miniflux", payload: map[string]any{"base_url": "", "api_token": "", "fetch_limit": 100, "lookback_hours": 24, "is_enabled": true}},
		{profileType: profile.TypePrompts, name: "default-prompts", payload: map[string]any{"target_language": "zh-CN", "translation_prompt": "", "analysis_prompt": "", "digest_prompt": "", "is_enabled": true}},
		{profileType: profile.TypePublish, name: "default-publish", payload: map[string]any{"target_type": "holo", "endpoint": "", "auth_token": "", "content_format": "markdown", "is_enabled": true}},
		{profileType: profile.TypeScheduler, name: "default-scheduler", payload: map[string]any{"schedule_enabled": true, "schedule_time": "07:00", "timezone": "Asia/Shanghai"}},
	}
	for _, def := range defaults {
		if _, err := s.repo.GetActive(ctx, def.profileType); err == nil {
			continue
		}
		payload, err := json.Marshal(def.payload)
		if err != nil { return err }
		if err := s.repo.Create(ctx, profile.Version{ProfileType: def.profileType, Name: def.name, Version: 1, IsActive: true, PayloadJSON: payload}); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service -run 'TestAdminConfigService|TestProfileService' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/profile/entity.go internal/service/profile_service.go internal/service/profile_service_test.go internal/service/admin_config_service.go internal/service/admin_config_service_test.go
git commit -m "feat: add admin config service"
```

### Task 2: Add job run persistence and status/test services

**Files:**
- Create: `migrations/0004_job_runs.up.sql`
- Create: `migrations/0004_job_runs.down.sql`
- Create: `internal/repository/postgres/models/job_run.go`
- Create: `internal/repository/postgres/job_run_repository.go`
- Create: `internal/repository/postgres/job_run_repository_test.go`
- Create: `internal/service/admin_status_service.go`
- Create: `internal/service/admin_status_service_test.go`
- Create: `internal/service/admin_test_service.go`
- Create: `internal/service/admin_test_service_test.go`
- Create: `internal/service/job_run_query_service.go`
- Create: `internal/service/job_run_query_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestJobRunRepositoryCreateAndListLatest(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	_ = db.AutoMigrate(&models.JobRunModel{})
	repo := postgres.NewJobRunRepository(db)

	if err := repo.Create(context.Background(), service.JobRunRecord{JobType: "daily_digest_run", Status: "succeeded", DigestDate: "2026-04-11"}); err != nil { t.Fatal(err) }
	runs, err := repo.ListLatest(context.Background(), service.JobRunListFilter{Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(runs) != 1 || runs[0].JobType != "daily_digest_run" { t.Fatalf("unexpected runs %#v", runs) }
}
```

```go
func TestAdminStatusServiceBuildsDashboardState(t *testing.T) {
	configs := adminConfigSnapshotStub{snapshot: service.AdminConfigSnapshot{LLM: service.LLMConfigView{BaseURL: "https://llm.local/v1", APIKey: service.SecretView{IsSet: true}}}}
	jobs := jobRunRepoStub{latestByType: map[string]service.JobRunRecord{"llm_test": {JobType: "llm_test", Status: "ok"}, "daily_digest_run": {JobType: "daily_digest_run", Status: "succeeded", DigestDate: "2026-04-11"}}}
	svc := service.NewAdminStatusService(configs, jobs)

	status, err := svc.GetStatus(context.Background())
	if err != nil { t.Fatal(err) }
	if !status.Integrations.LLM.Configured { t.Fatal("expected llm configured") }
	if status.Runtime.LatestJobStatus != "succeeded" { t.Fatalf("want succeeded got %q", status.Runtime.LatestJobStatus) }
}
```

```go
func TestAdminTestServiceRecordsLLMResult(t *testing.T) {
	checker := llmCheckerStub{latency: 850 * time.Millisecond}
	repo := &jobRunRepoStub{}
	svc := service.NewAdminTestService(checker, minifluxCheckerStub{}, publishCheckerStub{}, repo)

	result, err := svc.TestLLM(context.Background(), service.LLMTestDraft{BaseURL: "https://llm.local/v1", Model: "gpt-4.1-mini", APIKey: "token"})
	if err != nil { t.Fatal(err) }
	if result.Status != "ok" { t.Fatalf("want ok got %q", result.Status) }
	if len(repo.created) != 1 || repo.created[0].JobType != "llm_test" { t.Fatalf("unexpected records %#v", repo.created) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/postgres ./internal/service -run 'TestJobRunRepository|TestAdminStatusService|TestAdminTestService' -v`
Expected: FAIL with missing `JobRunModel`, `NewJobRunRepository`, `NewAdminStatusService`, and `NewAdminTestService`.

- [ ] **Step 3: Write minimal implementation**

```sql
CREATE TABLE job_runs (
  id VARCHAR(36) PRIMARY KEY,
  job_type TEXT NOT NULL,
  trigger_source TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  digest_date DATE,
  detail_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  error_message TEXT NOT NULL DEFAULT '',
  requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE INDEX idx_job_runs_type_requested_at ON job_runs (job_type, requested_at DESC);
```

```go
package models

type JobRunModel struct {
	ID            string     `gorm:"primaryKey;size:36"`
	JobType       string     `gorm:"not null;index:idx_job_runs_type_requested_at,priority:1"`
	TriggerSource string     `gorm:"not null;default:''"`
	Status        string     `gorm:"not null"`
	DigestDate    *time.Time `gorm:"type:date"`
	DetailJSON    []byte     `gorm:"type:jsonb;not null"`
	ErrorMessage  string     `gorm:"not null;default:''"`
	RequestedAt   time.Time  `gorm:"not null;autoCreateTime"`
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

func (JobRunModel) TableName() string { return "job_runs" }
```

```go
func (r *JobRunRepository) LatestByType(ctx context.Context, jobType string) (service.JobRunRecord, error) {
	var model models.JobRunModel
	if err := r.db.WithContext(ctx).Where("job_type = ?", jobType).Order("requested_at desc").First(&model).Error; err != nil {
		return service.JobRunRecord{}, err
	}
	return toJobRunRecord(model)
}

func (r *JobRunRepository) ListLatest(ctx context.Context, filter service.JobRunListFilter) ([]service.JobRunRecord, error) {
	var modelsOut []models.JobRunModel
	limit := filter.Limit
	if limit == 0 { limit = 20 }
	if err := r.db.WithContext(ctx).Order("requested_at desc").Limit(limit).Find(&modelsOut).Error; err != nil {
		return nil, err
	}
	return mapJobRuns(modelsOut)
}
```

```go
func (s *AdminStatusService) GetStatus(ctx context.Context) (AdminStatusView, error) {
	snapshot, err := s.configs.GetSnapshot(ctx)
	if err != nil { return AdminStatusView{}, err }
	latestRun, _ := s.jobs.LatestByType(ctx, "daily_digest_run")
	latestLLMTest, _ := s.jobs.LatestByType(ctx, "llm_test")
	return AdminStatusView{Integrations: IntegrationStatusView{LLM: IntegrationState{Configured: snapshot.LLM.BaseURL != "" && snapshot.LLM.APIKey.IsSet, LastTestStatus: latestLLMTest.Status, LastTestAt: latestLLMTest.FinishedAt}}, Runtime: RuntimeStatusView{LatestDigestDate: latestRun.DigestDate, LatestJobStatus: latestRun.Status}}, nil
}
```

```go
func (s *AdminTestService) TestLLM(ctx context.Context, draft LLMTestDraft) (ConnectivityTestResult, error) {
	latency, err := s.llm.Check(ctx, draft)
	result := ConnectivityTestResult{Status: "ok", Message: "connection succeeded", LatencyMS: latency.Milliseconds()}
	if err != nil { result.Status = "error"; result.Message = err.Error() }
	_ = s.jobs.Create(ctx, JobRunRecord{JobType: "llm_test", Status: result.Status, FinishedAt: time.Now(), Detail: map[string]any{"message": result.Message, "latency_ms": result.LatencyMS}})
	if err != nil { return result, err }
	return result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/repository/postgres ./internal/service -run 'TestJobRunRepository|TestAdminStatusService|TestAdminTestService' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add migrations/0004_job_runs.up.sql migrations/0004_job_runs.down.sql internal/repository/postgres/models/job_run.go internal/repository/postgres/job_run_repository.go internal/repository/postgres/job_run_repository_test.go internal/service/admin_status_service.go internal/service/admin_status_service_test.go internal/service/admin_test_service.go internal/service/admin_test_service_test.go internal/service/job_run_query_service.go internal/service/job_run_query_service_test.go
git commit -m "feat: add admin status and job run history"
```

### Task 3: Expose Admin API and load runtime config from DB

**Files:**
- Create: `internal/app/api/handlers/admin_handler.go`
- Create: `internal/app/api/handlers/admin_handler_test.go`
- Modify: `internal/app/api/router.go`
- Modify: `internal/app/api/router_test.go`
- Modify: `cmd/rss-api/main.go`
- Modify: `cmd/rss-worker/main.go`
- Modify: `cmd/rss-scheduler/main.go`
- Modify: `internal/app/scheduler/server.go`
- Create: `internal/service/runtime_config_service.go`
- Create: `internal/service/runtime_config_service_test.go`
- Modify: `api/openapi/openapi.yaml`

- [ ] **Step 1: Write the failing tests**

```go
func TestAdminStatusRouteReturnsDashboardJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{StatusReader: adminStatusStub{status: service.AdminStatusView{Runtime: service.RuntimeStatusView{LatestDigestDate: "2026-04-11", LatestJobStatus: "succeeded"}}}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK { t.Fatalf("want 200 got %d", rec.Code) }
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"latest_digest_date":"2026-04-11"`)) { t.Fatalf("unexpected body %s", rec.Body.String()) }
}
```

```go
func TestRuntimeConfigServiceUsesProfilePayloadBeforeEnvDefaults(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{profile.TypeLLM: {ProfileType: profile.TypeLLM, PayloadJSON: []byte(`{"base_url":"https://db-llm.local/v1","model":"gpt-4.1-mini","api_key":"db-key"}`)}}}
	defaults := &config.Config{}
	defaults.LLM.BaseURL = "https://env-llm.local/v1"
	svc := service.NewRuntimeConfigService(repo, defaults)

	snapshot, err := svc.Snapshot(context.Background())
	if err != nil { t.Fatal(err) }
	if snapshot.LLM.BaseURL != "https://db-llm.local/v1" { t.Fatalf("unexpected llm base url %q", snapshot.LLM.BaseURL) }
}
```

```go
func TestSchedulerLoopTriggersOncePerDigestDate(t *testing.T) {
	clock := newFakeClock([]time.Time{mustRFC3339("2026-04-11T06:59:00+08:00"), mustRFC3339("2026-04-11T07:00:00+08:00"), mustRFC3339("2026-04-11T07:00:30+08:00")})
	provider := schedulerConfigProviderStub{config: service.SchedulerRuntimeConfig{Enabled: true, ScheduleTime: "07:00", Timezone: "Asia/Shanghai"}}
	trigger := &schedulerTriggerStub{}
	server := scheduler.NewServer(provider, trigger, clock.Now, clock.Tick)

	server.RunSteps(context.Background(), 3)
	if len(trigger.calls) != 1 { t.Fatalf("want 1 trigger got %d", len(trigger.calls)) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/api/handlers ./internal/app/api ./internal/service ./internal/app/scheduler ./cmd/rss-worker ./cmd/rss-scheduler -run 'TestAdminStatusRouteReturnsDashboardJSON|TestRuntimeConfigServiceUsesProfilePayloadBeforeEnvDefaults|TestSchedulerLoopTriggersOncePerDigestDate' -v`
Expected: FAIL with missing admin routes, runtime config service, and scheduler loop constructor.

- [ ] **Step 3: Write minimal implementation**

```go
package handlers

type AdminDeps struct {
	StatusReader AdminStatusReader
	ConfigReader AdminConfigReader
	ConfigWriter AdminConfigWriter
	TestRunner   AdminTestRunner
	JobRunner    AdminJobRunner
	JobReader    JobRunReader
}
```

```go
func respondJSON[T any](c *gin.Context, fn func(context.Context) (T, error)) {
	out, err := fn(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func bindAndRun[T any, R any](c *gin.Context, payload *T, fn func(context.Context, T) (R, error)) {
	if err := c.ShouldBindJSON(payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := fn(c.Request.Context(), *payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
```

```go
func RegisterAdminRoutes(group *gin.RouterGroup, deps AdminDeps) {
	admin := group.Group("/admin")
	admin.GET("/status", func(c *gin.Context) { respondJSON(c, deps.StatusReader.GetStatus) })
	admin.GET("/configs", func(c *gin.Context) { respondJSON(c, deps.ConfigReader.GetSnapshot) })
	admin.PUT("/configs/llm", func(c *gin.Context) { bindAndRun(c, &service.UpdateLLMConfigInput{}, deps.ConfigWriter.UpdateLLM) })
	admin.PUT("/configs/miniflux", func(c *gin.Context) { bindAndRun(c, &service.UpdateMinifluxConfigInput{}, deps.ConfigWriter.UpdateMiniflux) })
	admin.PUT("/configs/prompts", func(c *gin.Context) { bindAndRun(c, &service.UpdatePromptConfigInput{}, deps.ConfigWriter.UpdatePrompts) })
	admin.PUT("/configs/publish", func(c *gin.Context) { bindAndRun(c, &service.UpdatePublishConfigInput{}, deps.ConfigWriter.UpdatePublish) })
	admin.PUT("/configs/scheduler", func(c *gin.Context) { bindAndRun(c, &service.UpdateSchedulerConfigInput{}, deps.ConfigWriter.UpdateScheduler) })
	admin.POST("/test/llm", func(c *gin.Context) { bindAndRun(c, &service.LLMTestRequest{}, deps.TestRunner.TestLLM) })
	admin.POST("/test/miniflux", func(c *gin.Context) { bindAndRun(c, &service.MinifluxTestRequest{}, deps.TestRunner.TestMiniflux) })
	admin.POST("/test/publish", func(c *gin.Context) { bindAndRun(c, &service.PublishTestRequest{}, deps.TestRunner.TestPublish) })
	admin.POST("/jobs/daily-digest/run", func(c *gin.Context) { bindAndRun(c, &service.AdminRunDailyDigestRequest{}, deps.JobRunner.RunDailyDigest) })
	admin.GET("/jobs", func(c *gin.Context) { respondJSON(c, deps.JobReader.ListJobRuns) })
}
```

```go
func (s *RuntimeConfigService) Snapshot(ctx context.Context) (RuntimeSnapshot, error) {
	llmPayload, _, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil { return RuntimeSnapshot{}, err }
	return RuntimeSnapshot{LLM: LLMRuntimeConfig{BaseURL: firstNonEmpty(stringValue(llmPayload, "base_url"), s.defaults.LLM.BaseURL), APIKey: firstNonEmpty(stringValue(llmPayload, "api_key"), s.defaults.LLM.APIKey), Model: firstNonEmpty(stringValue(llmPayload, "model"), s.defaults.LLM.Model)}}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" { return value }
	}
	return ""
}
```

```go
type Server struct { provider ConfigProvider; trigger Trigger; now func() time.Time; tick func() <-chan time.Time; lastDate string }

func (s *Server) handleTick(ctx context.Context, current time.Time) {
	cfg, err := s.provider.SchedulerConfig(ctx)
	if err != nil || !cfg.Enabled { return }
	loc, _ := time.LoadLocation(cfg.Timezone)
	localNow := current.In(loc)
	if localNow.Format("15:04") != cfg.ScheduleTime { return }
	digestDate := localNow.Format("2006-01-02")
	if digestDate == s.lastDate { return }
	s.lastDate = digestDate
	_ = s.trigger.TriggerDailyDigest(ctx, localNow)
}
```

```yaml
  /api/v1/admin/status:
    get:
      summary: Get dashboard status
  /api/v1/admin/configs/llm:
    put:
      summary: Update active LLM config
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/api/handlers ./internal/app/api ./internal/service ./internal/app/scheduler ./cmd/rss-worker ./cmd/rss-scheduler -run 'TestAdminStatusRouteReturnsDashboardJSON|TestRuntimeConfigServiceUsesProfilePayloadBeforeEnvDefaults|TestSchedulerLoopTriggersOncePerDigestDate' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/api/handlers/admin_handler.go internal/app/api/handlers/admin_handler_test.go internal/app/api/router.go internal/app/api/router_test.go cmd/rss-api/main.go cmd/rss-worker/main.go cmd/rss-scheduler/main.go internal/app/scheduler/server.go internal/service/runtime_config_service.go internal/service/runtime_config_service_test.go api/openapi/openapi.yaml
git commit -m "feat: expose admin api and db-backed runtime config"
```

### Task 4: Scaffold the React workspace and shared API client

**Files:**
- Modify: `.gitignore`
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/app/providers/AppProviders.tsx`
- Create: `web/src/app/router/index.tsx`
- Create: `web/src/app/layout/AppLayout.tsx`
- Create: `web/src/services/api/admin.ts`
- Create: `web/src/types/admin.ts`
- Create: `web/src/styles/index.css`

- [ ] **Step 1: Write the failing test**

```tsx
test('renders dashboard navigation item', async () => {
  render(
    <AppProviders>
      <MemoryRouter initialEntries={["/dashboard"]}>
        <AppRouter />
      </MemoryRouter>
    </AppProviders>,
  );

  expect(await screen.findByText('Dashboard')).toBeInTheDocument();
  expect(screen.getByText('FluxDigest')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm --prefix web run test -- --run`
Expected: FAIL with `Missing script: test` or missing React source files.

- [ ] **Step 3: Write minimal implementation**

```json
{
  "name": "fluxdigest-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "test": "vitest",
    "preview": "vite preview"
  },
  "dependencies": {
    "@ant-design/icons": "^6.0.0",
    "@tanstack/react-query": "^5.80.0",
    "antd": "^5.25.0",
    "react": "^19.1.0",
    "react-dom": "^19.1.0",
    "react-hook-form": "^7.57.0",
    "react-router-dom": "^7.6.0",
    "zod": "^4.0.0"
  }
}
```

```tsx
export function AppLayout() {
  const items = [
    { key: '/dashboard', label: <Link to="/dashboard">Dashboard</Link> },
    { key: '/configs/llm', label: <Link to="/configs/llm">LLM</Link> },
    { key: '/configs/miniflux', label: <Link to="/configs/miniflux">Miniflux</Link> },
    { key: '/configs/prompts', label: <Link to="/configs/prompts">Prompts</Link> },
    { key: '/configs/publish', label: <Link to="/configs/publish">Publish</Link> },
    { key: '/jobs', label: <Link to="/jobs">Jobs</Link> },
  ];
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Sider width={240} theme="light">
        <div className="brand">FluxDigest</div>
        <Menu mode="inline" items={items} />
      </Layout.Sider>
      <Layout>
        <Layout.Header className="topbar">Configuration Console</Layout.Header>
        <Layout.Content style={{ padding: 24 }}><Outlet /></Layout.Content>
      </Layout>
    </Layout>
  );
}
```

```ts
export async function getAdminStatus(): Promise<AdminStatus> {
  const response = await fetch('/api/v1/admin/status');
  if (!response.ok) throw new Error(`failed to fetch admin status: ${response.status}`);
  return response.json();
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm --prefix web install`
Expected: PASS

Run: `npm --prefix web run test -- --run`
Expected: PASS

Run: `npm --prefix web run build`
Expected: PASS and generate `web/dist`

- [ ] **Step 5: Commit**

```bash
git add .gitignore web/package.json web/vite.config.ts web/tsconfig.json web/index.html web/src/main.tsx web/src/app/providers/AppProviders.tsx web/src/app/router/index.tsx web/src/app/layout/AppLayout.tsx web/src/services/api/admin.ts web/src/types/admin.ts web/src/styles/index.css
git commit -m "feat: scaffold webui app shell"
```

### Task 5: Build dashboard, jobs, and config pages

**Files:**
- Create: `web/src/components/common/PageHeader.tsx`
- Create: `web/src/components/status/StatusBadge.tsx`
- Create: `web/src/components/forms/SecretField.tsx`
- Create: `web/src/components/jobs/JobRunDrawer.tsx`
- Create: `web/src/services/queries/admin.ts`
- Create: `web/src/services/mutations/admin.ts`
- Create: `web/src/pages/dashboard/DashboardPage.tsx`
- Create: `web/src/pages/jobs/JobsPage.tsx`
- Create: `web/src/pages/configs/llm/LLMConfigPage.tsx`
- Create: `web/src/pages/configs/miniflux/MinifluxConfigPage.tsx`
- Create: `web/src/pages/configs/prompts/PromptConfigPage.tsx`
- Create: `web/src/pages/configs/publish/PublishConfigPage.tsx`
- Create: `web/src/__tests__/dashboard.test.tsx`
- Create: `web/src/__tests__/jobs.test.tsx`
- Create: `web/src/__tests__/configs.test.tsx`

- [ ] **Step 1: Write the failing tests**

```tsx
test('dashboard renders latest digest and quick actions', async () => {
  server.use(
    http.get('/api/v1/admin/status', () => HttpResponse.json({ integrations: { llm: { configured: true, last_test_status: 'ok' } }, runtime: { latest_digest_date: '2026-04-11', latest_job_status: 'succeeded' }, system: { api: 'ok', db: 'ok', redis: 'ok' } })),
    http.get('/api/v1/admin/jobs', () => HttpResponse.json({ items: [] })),
  );
  renderPage(<DashboardPage />);
  expect(await screen.findByText('2026-04-11')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '手动触发日报' })).toBeInTheDocument();
});
```

```tsx
test('jobs page opens detail drawer', async () => {
  server.use(
    http.get('/api/v1/admin/jobs', () => HttpResponse.json({ items: [{ id: 'job-1', job_type: 'daily_digest_run', status: 'succeeded', digest_date: '2026-04-11' }] })),
    http.get('/api/v1/admin/jobs/job-1', () => HttpResponse.json({ id: 'job-1', status: 'succeeded', detail: { remote_url: 'https://blog.local/post/1' } })),
  );
  renderPage(<JobsPage />);
  await userEvent.click(await screen.findByRole('button', { name: '查看详情' }));
  expect(await screen.findByText('https://blog.local/post/1')).toBeInTheDocument();
});
```

```tsx
test('llm config page saves keep-secret payload', async () => {
  const putSpy = vi.fn();
  server.use(
    http.get('/api/v1/admin/configs', () => HttpResponse.json({ llm: { base_url: 'https://llm.local/v1', model: 'gpt-4.1-mini', api_key: { is_set: true, masked_value: 'secr****' }, is_enabled: true, timeout_ms: 30000 } })),
    http.put('/api/v1/admin/configs/llm', async ({ request }) => { putSpy(await request.json()); return HttpResponse.json({ ok: true }); }),
  );
  renderPage(<LLMConfigPage />);
  await userEvent.clear(await screen.findByLabelText('Base URL'));
  await userEvent.type(screen.getByLabelText('Base URL'), 'https://proxy.local/v1');
  await userEvent.click(screen.getByRole('button', { name: '保存配置' }));
  expect(putSpy).toHaveBeenCalledWith(expect.objectContaining({ api_key: { mode: 'keep' } }));
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm --prefix web run test -- --run src/__tests__/dashboard.test.tsx src/__tests__/jobs.test.tsx src/__tests__/configs.test.tsx`
Expected: FAIL with missing pages, query hooks, mutation hooks, and shared components.

- [ ] **Step 3: Write minimal implementation**

```ts
export function useAdminStatus() { return useQuery({ queryKey: ['admin', 'status'], queryFn: getAdminStatus, staleTime: 30_000 }); }
export function useJobRuns() { return useQuery({ queryKey: ['admin', 'jobs'], queryFn: getJobRuns }); }
export function useRunDailyDigest() {
  const queryClient = useQueryClient();
  return useMutation({ mutationFn: runDailyDigest, onSuccess: async () => {
    await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'status'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'jobs'] })]);
    message.success('日报任务已触发');
  }});
}
```

```tsx
export function SecretField({ value, onChange }: SecretFieldProps) {
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Alert type={value.is_set ? 'success' : 'info'} message={value.is_set ? `已设置：${value.masked_value}` : '未设置'} />
      <Radio.Group value={value.mode} onChange={event => onChange({ ...value, mode: event.target.value })}>
        <Radio value="keep">保留</Radio>
        <Radio value="replace">替换</Radio>
        <Radio value="clear">清空</Radio>
      </Radio.Group>
      {value.mode === 'replace' ? <Input.Password aria-label="Secret Value" onChange={event => onChange({ ...value, value: event.target.value })} /> : null}
    </Space>
  );
}
```

```tsx
export function DashboardPage() {
  const status = useAdminStatus();
  const jobs = useJobRuns();
  return (
    <Space direction="vertical" size={16} style={{ display: 'flex' }}>
      <PageHeader title="Dashboard" subtitle="查看系统状态与快捷操作" />
      <Row gutter={[16, 16]}>
        <Col span={8}><Card title="今日日报">{status.data?.runtime.latest_digest_date ?? '—'}</Card></Col>
        <Col span={8}><Card title="最近任务"><StatusBadge status={status.data?.runtime.latest_job_status ?? 'unknown'} /></Card></Col>
        <Col span={8}><Card title="快速操作"><Button type="primary">手动触发日报</Button></Card></Col>
      </Row>
      <Card title="最近任务"><Table rowKey="id" dataSource={jobs.data?.items ?? []} columns={[{ title: '类型', dataIndex: 'job_type' }, { title: '状态', dataIndex: 'status', render: value => <StatusBadge status={value} /> }]} pagination={false} /></Card>
    </Space>
  );
}
```

```tsx
export function LLMConfigPage() {
  const snapshot = useAdminConfigs();
  const saveMutation = useSaveLLMConfig();
  const testMutation = useTestLLMConfig();
  const form = useForm<LLMConfigFormValues>();
  useEffect(() => { if (snapshot.data?.llm) { form.reset({ ...snapshot.data.llm, api_key: { mode: 'keep', is_set: snapshot.data.llm.api_key.is_set, masked_value: snapshot.data.llm.api_key.masked_value } }); } }, [snapshot.data, form]);
  return (
    <Form layout="vertical" onFinish={form.handleSubmit(values => saveMutation.mutate(values))}>
      <Form.Item label="Base URL"><Controller name="base_url" control={form.control} render={({ field }) => <Input aria-label="Base URL" {...field} />} /></Form.Item>
      <Form.Item label="Model"><Controller name="model" control={form.control} render={({ field }) => <Input {...field} />} /></Form.Item>
      <Form.Item label="API Key"><Controller name="api_key" control={form.control} render={({ field }) => <SecretField value={field.value} onChange={field.onChange} />} /></Form.Item>
      <Space><Button onClick={form.handleSubmit(values => testMutation.mutate(values))}>测试连接</Button><Button type="primary" htmlType="submit">保存配置</Button></Space>
    </Form>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm --prefix web run test -- --run src/__tests__/dashboard.test.tsx src/__tests__/jobs.test.tsx src/__tests__/configs.test.tsx`
Expected: PASS

Run: `npm --prefix web run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/common/PageHeader.tsx web/src/components/status/StatusBadge.tsx web/src/components/forms/SecretField.tsx web/src/components/jobs/JobRunDrawer.tsx web/src/services/queries/admin.ts web/src/services/mutations/admin.ts web/src/pages/dashboard/DashboardPage.tsx web/src/pages/jobs/JobsPage.tsx web/src/pages/configs/llm/LLMConfigPage.tsx web/src/pages/configs/miniflux/MinifluxConfigPage.tsx web/src/pages/configs/prompts/PromptConfigPage.tsx web/src/pages/configs/publish/PublishConfigPage.tsx web/src/__tests__/dashboard.test.tsx web/src/__tests__/jobs.test.tsx web/src/__tests__/configs.test.tsx
git commit -m "feat: add webui pages and forms"
```

### Task 6: Serve SPA from rss-api and verify delivery

**Files:**
- Modify: `internal/app/api/router.go`
- Modify: `internal/app/api/router_test.go`
- Modify: `deployments/docker/api.Dockerfile`
- Modify: `deployments/compose/docker-compose.yml`
- Modify: `scripts/smoke-compose.ps1`

- [ ] **Step 1: Write the failing test**

```go
func TestRouterServesStaticIndexForNonAPIRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><body>FluxDigest UI</body></html>"), 0o644)
	router := api.NewRouter(api.WithStaticDir(dir))

	req := httptest.NewRequest(http.MethodGet, "/configs/llm", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK { t.Fatalf("want 200 got %d", rec.Code) }
	if !bytes.Contains(rec.Body.Bytes(), []byte("FluxDigest UI")) { t.Fatalf("unexpected body %s", rec.Body.String()) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/api -run TestRouterServesStaticIndexForNonAPIRoute -v`
Expected: FAIL until static fallback is enabled.

- [ ] **Step 3: Write minimal implementation**

```go
func registerStaticRoutes(router *gin.Engine, staticDir string) {
	if staticDir == "" { return }
	indexFile := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil { return }
	router.Static("/assets", filepath.Join(staticDir, "assets"))
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/healthz" || c.Request.URL.Path == "/metrics" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(indexFile)
	})
}
```

```dockerfile
FROM node:22-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web .
RUN npm run build

FROM golang:1.24 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/rss-api ./cmd/rss-api

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/rss-api /app/rss-api
COPY --from=builder /src/configs /app/configs
COPY --from=builder /src/migrations /app/migrations
COPY --from=web-builder /src/web/dist /app/web/dist
ENV APP_STATIC_DIR=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["/app/rss-api"]
```

```powershell
npm --prefix web install
npm --prefix web run build
$composeFile = "deployments/compose/docker-compose.yml"
docker compose -f $composeFile build rss-api
$adminStatus = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/admin/status"
if (-not $adminStatus.runtime) { throw "admin status missing runtime" }
$indexHtml = Invoke-WebRequest -Uri "http://127.0.0.1:8080/dashboard"
if ($indexHtml.Content -notmatch "FluxDigest") { throw "spa index missing FluxDigest" }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/api -run TestRouterServesStaticIndexForNonAPIRoute -v`
Expected: PASS

Run: `npm --prefix web run build`
Expected: PASS

Run: `go test ./...`
Expected: PASS

Run: `go build ./cmd/...`
Expected: PASS

Run: `docker compose -f deployments/compose/docker-compose.yml config`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/api/router.go internal/app/api/router_test.go deployments/docker/api.Dockerfile deployments/compose/docker-compose.yml scripts/smoke-compose.ps1
git commit -m "feat: ship webui with api image"
```
