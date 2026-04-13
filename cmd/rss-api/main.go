package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	jobRunRepo := &adminJobRunRepoAdapter{repo: postgresrepo.NewJobRunRepository(db)}
	articleQueryService := service.NewArticleQueryService(db)
	dossierQueryService := service.NewDossierQueryService(db)
	digestQueryService := service.NewDigestQueryService(db)
	profileQueryService := service.NewProfileQueryService(db)
	adminConfigService := service.NewAdminConfigService(profileRepo)
	adminStatusService := service.NewAdminStatusServiceWithDigest(adminConfigService, jobRunRepo, digestQueryService)
	adminTestService := service.NewAdminTestService(newAdminLLMConnectivityChecker(defaultAdminLLMTestTimeout), nil, nil, jobRunRepo)
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
			LLMTester:  adminTestService,
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
