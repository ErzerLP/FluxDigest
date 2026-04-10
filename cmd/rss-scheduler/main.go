package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	appscheduler "rss-platform/internal/app/scheduler"
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

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.Redis.Addr})
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close asynq client: %v", err)
		}
	}()

	queue := dailyDigestQueue{client: client, queue: cfg.Job.Queue}
	cron := appscheduler.Start(service.NewJobService(queue))
	log.Println("rss-scheduler started")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	<-cron.Stop().Done()
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
