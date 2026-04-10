package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hibiken/asynq"

	"rss-platform/internal/app/api"
	"rss-platform/internal/config"
	"rss-platform/internal/service"
	asynqtask "rss-platform/internal/task/asynq"
)

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
	router := api.NewRouter(
		api.WithAPIKey(cfg.Job.APIKey),
		api.WithJobTrigger(service.NewJobService(queue)),
	)

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	if err := router.Run(addr); err != nil {
		log.Fatalf("run api: %v", err)
	}
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
