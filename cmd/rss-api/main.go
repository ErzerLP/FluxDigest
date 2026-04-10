package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	_ "github.com/jackc/pgx/v5/stdlib"

	"rss-platform/internal/app/api"
	"rss-platform/internal/config"
	"rss-platform/internal/service"
	asynqtask "rss-platform/internal/task/asynq"
	"rss-platform/internal/telemetry"
)

var errDatabaseDSNRequired = errors.New("APP_DATABASE_DSN is required")

type dbCloser interface {
	Close() error
}

type connectPostgresFunc func(ctx context.Context, dsn string) (dbCloser, error)

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

	db, err := connect(ctx, cfg.Database.DSN)
	if err != nil {
		return nil, nil, err
	}

	router := api.NewRouter(
		api.WithAPIKey(cfg.Job.APIKey),
		api.WithMetrics(metrics),
		api.WithJobTrigger(service.NewJobService(queue, metrics)),
	)

	return router, db, nil
}

func connectPostgres(ctx context.Context, dsn string) (dbCloser, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
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

	_, err = q.client.EnqueueContext(ctx, task, asynq.Queue(q.queue))
	return err
}
