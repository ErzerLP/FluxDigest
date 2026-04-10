package main

import (
	"context"
	"log"

	"github.com/hibiken/asynq"

	appworker "rss-platform/internal/app/worker"
	"rss-platform/internal/config"
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

	server := appworker.NewServer(asynq.RedisClientOpt{Addr: cfg.Redis.Addr}, appworker.ServerConfig{
		Concurrency: cfg.Worker.Concurrency,
		Queues: map[string]int{
			cfg.Job.Queue: 1,
		},
	})
	mux := appworker.NewServeMux(nil, asynqtask.NewDailyDigestHandler(func(_ context.Context, digestDate string) error {
		log.Printf("daily digest task consumed: %s", digestDate)
		return nil
	}))

	log.Println("rss-worker started")
	if err := server.Run(mux); err != nil {
		log.Fatalf("run worker: %v", err)
	}
}
