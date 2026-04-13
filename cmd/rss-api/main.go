package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	llmadapter "rss-platform/internal/adapter/llm"
	"rss-platform/internal/app/api"
	"rss-platform/internal/app/api/handlers"
	"rss-platform/internal/app/api/middleware"
	"rss-platform/internal/config"
	postgresrepo "rss-platform/internal/repository/postgres"
	"rss-platform/internal/security"
	"rss-platform/internal/service"
	asynqtask "rss-platform/internal/task/asynq"
	"rss-platform/internal/telemetry"
)

var errDatabaseDSNRequired = errors.New("APP_DATABASE_DSN is required")

const defaultAdminLLMTestTimeout = 30 * time.Second
const maxAdminLLMTestTimeoutMS = 2_147_483_647

type dbCloser interface {
	Close() error
}

type multiCloser struct {
	closers []dbCloser
}

func (c multiCloser) Close() error {
	var firstErr error
	for _, closer := range c.closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type connectPostgresFunc func(ctx context.Context, dsn string) (*gorm.DB, dbCloser, error)
type chatModelFactory func(ctx context.Context, cfg llmadapter.FactoryConfig) (model.BaseChatModel, error)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.Redis.Addr == "" {
		log.Fatal("APP_REDIS_ADDR is required")
	}
	if cfg.Job.APIKey == "" {
		log.Fatal("APP_JOB_API_KEY is required")
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.Redis.Addr})
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close asynq client: %v", err)
		}
	}()

	queue := dailyDigestQueue{client: client, queue: cfg.Job.Queue}
	metrics := telemetry.NewMetrics()
	router, db, err := buildAPIRouter(context.Background(), cfg, queue, queue, connectPostgres, metrics)
	if err != nil {
		log.Fatalf("build api router: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close api resources: %v", err)
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	if err := router.Run(addr); err != nil {
		log.Fatalf("run api: %v", err)
	}
}

func buildAPIRouter(ctx context.Context, cfg *config.Config, dailyQueue service.DailyDigestQueue, articleQueue service.ArticleReprocessQueue, connect connectPostgresFunc, metrics *telemetry.Metrics) (*gin.Engine, dbCloser, error) {
	if cfg == nil {
		return nil, nil, errors.New("config is required")
	}
	if cfg.Database.DSN == "" {
		return nil, nil, errDatabaseDSNRequired
	}
	if connect == nil {
		connect = connectPostgres
	}
	if metrics == nil {
		metrics = telemetry.NewMetrics()
	}

	db, closer, err := connect(ctx, cfg.Database.DSN)
	if err != nil {
		return nil, nil, err
	}

	if err := ensureRuntimeState(ctx, db); err != nil {
		_ = closer.Close()
		return nil, nil, err
	}

	profileRepo := postgresrepo.NewProfileRepository(db)
	adminSecretCipher, err := newAdminSecretCipher(cfg)
	if err != nil {
		_ = closer.Close()
		return nil, nil, err
	}
	jobRunRepo := &adminJobRunRepoAdapter{repo: postgresrepo.NewJobRunRepository(db)}
	articleQueryService := service.NewArticleQueryService(db)
	dossierQueryService := service.NewDossierQueryService(db)
	digestQueryService := service.NewDigestQueryService(db)
	profileQueryService := service.NewProfileQueryService(db)
	runtimeConfigs := service.NewRuntimeConfigService(profileRepo, cfg)
	adminConfigService := service.NewAdminConfigService(profileRepo, adminSecretCipher, cfg)
	adminStatusService := service.NewAdminStatusServiceWithDigest(adminConfigService, jobRunRepo, digestQueryService)
	adminTestService := service.NewAdminTestService(
		newAdminLLMConnectivityChecker(defaultAdminLLMTestTimeout),
		newAdminMinifluxConnectivityChecker(runtimeConfigs),
		newAdminPublishConnectivityChecker(runtimeConfigs),
		jobRunRepo,
	)
	jobRunQueryService := service.NewJobRunQueryService(db)
	adminUserRepo := postgresrepo.NewAdminUserRepository(db)
	adminSessionStore, adminSessionCloser := newAdminSessionStore(cfg)
	adminAuthService := service.NewAdminAuthService(adminUserRepo, adminSessionStore)
	adminSessionMiddleware := middleware.RequireAdminSession(adminAuthService, middleware.AdminSessionOptions{
		CookieName: service.DefaultAdminSessionCookieName,
	})
	router := api.NewRouter(
		api.WithAPIKey(cfg.Job.APIKey),
		api.WithMetrics(metrics),
		api.WithArticleReader(articleQueryService),
		api.WithDossierReader(dossierQueryService),
		api.WithDigestReader(digestQueryService),
		api.WithProfileReader(profileQueryService),
		api.WithJobTrigger(service.NewJobService(dailyQueue, articleQueue, metrics)),
		api.WithAdminDeps(handlers.AdminDeps{
			Status:     adminStatusService,
			Configs:    adminConfigService,
			LLMUpdater: adminConfigService,
			Tester:     adminTestService,
			Jobs:       jobRunQueryService,
		}),
		api.WithAdminAuthDeps(handlers.AdminAuthDeps{
			Auth:        adminAuthService,
			CookieName:  service.DefaultAdminSessionCookieName,
			CookiePath:  "/",
			SessionAuth: adminSessionMiddleware,
		}),
		api.WithAdminSessionMiddleware(adminSessionMiddleware),
	)

	return router, multiCloser{closers: []dbCloser{closer, adminSessionCloser}}, nil
}

func newAdminSessionStore(cfg *config.Config) (service.AdminSessionStore, dbCloser) {
	if cfg != nil && cfg.Redis.Addr != "" {
		store := service.NewRedisAdminSessionStore(cfg.Redis.Addr)
		return store, store
	}
	return service.NewInMemoryAdminSessionStore(), nil
}

func newAdminSecretCipher(cfg *config.Config) (*security.SecretCipher, error) {
	if cfg == nil || strings.TrimSpace(cfg.Security.SecretKey) == "" {
		return nil, nil
	}
	return security.NewSecretCipher(strings.TrimSpace(cfg.Security.SecretKey))
}

func connectPostgres(ctx context.Context, dsn string) (*gorm.DB, dbCloser, error) {
	db, err := postgresrepo.Open(dsn)
	if err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	return db, sqlDB, nil
}

func ensureRuntimeState(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	migrationsDir, err := resolveMigrationsDir()
	if err != nil {
		return err
	}

	bootstrap := service.NewRuntimeBootstrapService(
		postgresrepo.NewMigrator(sqlDB, migrationsDir),
		service.NewProfileService(postgresrepo.NewProfileRepository(db)),
		service.NewAdminUserService(postgresrepo.NewAdminUserRepository(db)),
	)

	return bootstrap.Ensure(ctx)
}

func resolveMigrationsDir() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(current, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", errors.New("migrations directory not found")
}

type dailyDigestQueue struct {
	client *asynq.Client
	queue  string
}

func (q dailyDigestQueue) EnqueueDailyDigest(ctx context.Context, digestDate string) error {
	return q.EnqueueDailyDigestWithOptions(ctx, digestDate, service.DailyDigestTriggerOptions{})
}

func (q dailyDigestQueue) EnqueueDailyDigestWithOptions(ctx context.Context, digestDate string, opts service.DailyDigestTriggerOptions) error {
	task, err := asynqtask.NewDailyDigestTask(asynqtask.DailyDigestPayload{
		DigestDate: digestDate,
		Force:      opts.Force,
	})
	if err != nil {
		return err
	}

	taskID := dailyDigestTaskID(digestDate)
	if opts.Force {
		taskID = forceDailyDigestTaskID(digestDate)
	}

	_, err = q.client.EnqueueContext(
		ctx,
		task,
		asynq.Queue(q.queue),
		asynq.TaskID(taskID),
	)
	return mapDailyDigestEnqueueError(err, opts.Force)
}

func (q dailyDigestQueue) EnqueueArticleReprocess(ctx context.Context, articleID string, force bool) error {
	task, err := asynqtask.NewReprocessArticleTask(asynqtask.ReprocessArticlePayload{
		ArticleID: articleID,
		Force:     force,
	})
	if err != nil {
		return err
	}

	taskID := articleReprocessTaskID(articleID)
	if force {
		taskID = forceArticleReprocessTaskID(articleID)
	}

	_, err = q.client.EnqueueContext(
		ctx,
		task,
		asynq.Queue(q.queue),
		asynq.TaskID(taskID),
	)
	return mapArticleReprocessEnqueueError(err, force)
}

func dailyDigestTaskID(digestDate string) string {
	return "daily-digest:" + digestDate
}

func forceDailyDigestTaskID(digestDate string) string {
	return "daily-digest:force:" + digestDate
}

func articleReprocessTaskID(articleID string) string {
	return "article-reprocess:" + articleID
}

func forceArticleReprocessTaskID(articleID string) string {
	return "article-reprocess:force:" + articleID
}

func mapDailyDigestEnqueueError(err error, force bool) error {
	if !force && errors.Is(err, asynq.ErrTaskIDConflict) {
		return service.ErrDailyDigestAlreadyQueued
	}
	return err
}

func mapArticleReprocessEnqueueError(err error, force bool) error {
	if !force && errors.Is(err, asynq.ErrTaskIDConflict) {
		return service.ErrArticleReprocessAlreadyQueued
	}
	return err
}

type adminLLMConnectivityChecker struct {
	timeout      time.Duration
	newChatModel chatModelFactory
}

func newAdminLLMConnectivityChecker(timeout time.Duration) adminLLMConnectivityChecker {
	return adminLLMConnectivityChecker{
		timeout:      timeout,
		newChatModel: llmadapter.NewChatModel,
	}
}

func (c adminLLMConnectivityChecker) Check(ctx context.Context, draft service.LLMTestDraft) (time.Duration, error) {
	startedAt := time.Now()

	timeout := resolveAdminLLMCheckTimeout(c.timeout, draft.TimeoutMS)

	newChatModel := c.newChatModel
	if newChatModel == nil {
		newChatModel = llmadapter.NewChatModel
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	chatModel, err := newChatModel(checkCtx, llmadapter.FactoryConfig{
		BaseURL: draft.BaseURL,
		APIKey:  draft.APIKey,
		Model:   draft.Model,
	})
	if err != nil {
		return time.Since(startedAt), err
	}

	_, err = chatModel.Generate(checkCtx, []*schema.Message{schema.UserMessage("ping")})
	if errors.Is(err, context.DeadlineExceeded) {
		return time.Since(startedAt), fmt.Errorf("llm connectivity test timed out after %s: %w", timeout, err)
	}

	return time.Since(startedAt), err
}

func resolveAdminLLMCheckTimeout(defaultTimeout time.Duration, timeoutMS int) time.Duration {
	if timeoutMS > 0 {
		return time.Duration(clampAdminLLMTestTimeoutMS(timeoutMS)) * time.Millisecond
	}
	if defaultTimeout > 0 {
		return defaultTimeout
	}
	return defaultAdminLLMTestTimeout
}

func clampAdminLLMTestTimeoutMS(timeoutMS int) int {
	if timeoutMS <= 0 {
		return 0
	}
	if timeoutMS > maxAdminLLMTestTimeoutMS {
		return maxAdminLLMTestTimeoutMS
	}
	return timeoutMS
}

type adminMinifluxRuntimeReader interface {
	Miniflux(ctx context.Context) (service.MinifluxRuntimeConfig, error)
}

type adminPublishRuntimeReader interface {
	Publish(ctx context.Context) (service.PublishRuntimeConfig, error)
}

type adminMinifluxConnectivityChecker struct {
	configs    adminMinifluxRuntimeReader
	httpClient *http.Client
}

func newAdminMinifluxConnectivityChecker(configs adminMinifluxRuntimeReader) adminMinifluxConnectivityChecker {
	return adminMinifluxConnectivityChecker{
		configs:    configs,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c adminMinifluxConnectivityChecker) Check(ctx context.Context) (time.Duration, error) {
	startedAt := time.Now()

	cfg, err := c.configs.Miniflux(ctx)
	if err != nil {
		return time.Since(startedAt), err
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return time.Since(startedAt), errors.New("miniflux base url is required")
	}
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return time.Since(startedAt), errors.New("miniflux auth token is required")
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.BaseURL, "/")+"/v1/entries?limit=1", nil)
	if err != nil {
		return time.Since(startedAt), err
	}
	req.Header.Set("X-Auth-Token", cfg.AuthToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return time.Since(startedAt), err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return time.Since(startedAt), fmt.Errorf("miniflux connectivity check failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return time.Since(startedAt), nil
}

type adminPublishConnectivityChecker struct {
	configs    adminPublishRuntimeReader
	httpClient *http.Client
}

func newAdminPublishConnectivityChecker(configs adminPublishRuntimeReader) adminPublishConnectivityChecker {
	return adminPublishConnectivityChecker{
		configs:    configs,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c adminPublishConnectivityChecker) Check(ctx context.Context) (time.Duration, error) {
	startedAt := time.Now()

	cfg, err := c.configs.Publish(ctx)
	if err != nil {
		return time.Since(startedAt), err
	}

	switch service.ResolvePublishProvider(cfg.Provider, cfg.HaloBaseURL, cfg.OutputDir) {
	case "halo":
		return c.checkHalo(ctx, startedAt, cfg)
	case "markdown_export":
		return c.checkMarkdownExport(ctx, startedAt, cfg)
	default:
		return time.Since(startedAt), fmt.Errorf("unsupported publish provider %q", cfg.Provider)
	}
}

func (c adminPublishConnectivityChecker) checkHalo(ctx context.Context, startedAt time.Time, cfg service.PublishRuntimeConfig) (time.Duration, error) {
	if strings.TrimSpace(cfg.HaloBaseURL) == "" {
		return time.Since(startedAt), errors.New("publish halo base url is required")
	}
	if strings.TrimSpace(cfg.HaloToken) == "" {
		return time.Since(startedAt), errors.New("publish halo token is required")
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.HaloBaseURL, "/")+"/apis/api.console.halo.run/v1alpha1/posts?page=1&size=1", nil)
	if err != nil {
		return time.Since(startedAt), err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.HaloToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return time.Since(startedAt), err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return time.Since(startedAt), fmt.Errorf("publish halo connectivity check failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return time.Since(startedAt), nil
}

func (c adminPublishConnectivityChecker) checkMarkdownExport(ctx context.Context, startedAt time.Time, cfg service.PublishRuntimeConfig) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return time.Since(startedAt), err
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		return time.Since(startedAt), errors.New("publish output dir is required")
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return time.Since(startedAt), err
	}
	return time.Since(startedAt), nil
}

type adminJobRunRepoAdapter struct {
	repo *postgresrepo.JobRunRepository
}

func (a *adminJobRunRepoAdapter) LatestByType(ctx context.Context, jobType string) (service.JobRunRecord, error) {
	if a == nil || a.repo == nil {
		return service.JobRunRecord{}, nil
	}

	record, err := a.repo.LatestByType(ctx, jobType)
	if err != nil {
		return service.JobRunRecord{}, err
	}

	return service.JobRunRecord{
		ID:            record.ID,
		JobType:       record.JobType,
		TriggerSource: record.TriggerSource,
		Status:        record.Status,
		DigestDate:    record.DigestDate,
		Detail:        record.Detail,
		ErrorMessage:  record.ErrorMessage,
		RequestedAt:   record.RequestedAt,
		StartedAt:     record.StartedAt,
		FinishedAt:    record.FinishedAt,
	}, nil
}

func (a *adminJobRunRepoAdapter) Create(ctx context.Context, record service.JobRunRecord) error {
	if a == nil || a.repo == nil {
		return nil
	}

	return a.repo.Create(ctx, postgresrepo.JobRunRecord{
		ID:            record.ID,
		JobType:       record.JobType,
		TriggerSource: record.TriggerSource,
		Status:        record.Status,
		DigestDate:    record.DigestDate,
		Detail:        record.Detail,
		ErrorMessage:  record.ErrorMessage,
		RequestedAt:   record.RequestedAt,
		StartedAt:     record.StartedAt,
		FinishedAt:    record.FinishedAt,
	})
}
