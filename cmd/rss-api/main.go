package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"rss-platform/internal/app/api"
	"rss-platform/internal/config"
	postgresrepo "rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"
	asynqtask "rss-platform/internal/task/asynq"
	"rss-platform/internal/telemetry"
)

var errDatabaseDSNRequired = errors.New("APP_DATABASE_DSN is required")

type dbCloser interface {
	Close() error
}

type connectPostgresFunc func(ctx context.Context, dsn string) (*gorm.DB, dbCloser, error)

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
	router, db, err := buildAPIRouter(context.Background(), cfg, queue, connectPostgres, metrics)
	if err != nil {
		log.Fatalf("build api router: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close postgres: %v", err)
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	if err := router.Run(addr); err != nil {
		log.Fatalf("run api: %v", err)
	}
}

func buildAPIRouter(ctx context.Context, cfg *config.Config, queue service.DailyDigestQueue, connect connectPostgresFunc, metrics *telemetry.Metrics) (*gin.Engine, dbCloser, error) {
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

	articleQueryService := service.NewArticleQueryService(db)
	digestQueryService := service.NewDigestQueryService(db)
	profileQueryService := service.NewProfileQueryService(db)
	router := api.NewRouter(
		api.WithAPIKey(cfg.Job.APIKey),
		api.WithMetrics(metrics),
		api.WithArticleReader(articleQueryService),
		api.WithDigestReader(digestQueryService),
		api.WithProfileReader(profileQueryService),
		api.WithJobTrigger(service.NewJobService(queue, metrics)),
	)

	return router, closer, nil
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
	task, err := asynqtask.NewDailyDigestTask(digestDate)
	if err != nil {
		return err
	}

	_, err = q.client.EnqueueContext(
		ctx,
		task,
		asynq.Queue(q.queue),
		asynq.TaskID(dailyDigestTaskID(digestDate)),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return service.ErrDailyDigestAlreadyQueued
	}
	return err
}

func dailyDigestTaskID(digestDate string) string {
	return "daily-digest:" + digestDate
}
