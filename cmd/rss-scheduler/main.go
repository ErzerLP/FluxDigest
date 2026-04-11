package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

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
	cron := appscheduler.Start(schedulerTrigger{job: service.NewJobService(queue)})
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

type schedulerTrigger struct {
	job *service.JobService
}

func (t schedulerTrigger) TriggerDailyDigest(ctx context.Context, now time.Time) error {
	if t.job == nil {
		return nil
	}

	_, err := t.job.TriggerDailyDigest(ctx, now)
	return err
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
