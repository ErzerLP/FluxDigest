package service

import (
	"context"
	"errors"
	"time"
)

var errDailyDigestQueueRequired = errors.New("daily digest queue is required")
var ErrDailyDigestAlreadyQueued = errors.New("daily digest already queued")

// JobTriggerResult 表示手动或计划触发日报后的真实受理状态。
type JobTriggerResult struct {
	DigestDate string
	Status     string
}

// DailyDigestQueue 定义日报任务入队所需的最小能力。
type DailyDigestQueue interface {
	EnqueueDailyDigest(ctx context.Context, digestDate string) error
}

// JobMetrics 定义作业指标上报所需的最小能力。
type JobMetrics interface {
	IncDailyDigestTriggered()
	IncDailyDigestSkipped()
}

// JobService 负责按 digestDate 做最小幂等触发。
type JobService struct {
	queue   DailyDigestQueue
	metrics JobMetrics
}

// NewJobService 创建 JobService。
func NewJobService(queue DailyDigestQueue, metrics ...JobMetrics) *JobService {
	svc := &JobService{queue: queue}
	if len(metrics) > 0 {
		svc.metrics = metrics[0]
	}
	return svc
}

// TriggerDailyDigest 按上海日历日触发日报任务，重复日期会被跳过。
func (s *JobService) TriggerDailyDigest(ctx context.Context, now time.Time) (JobTriggerResult, error) {
	if s.queue == nil {
		return JobTriggerResult{}, errDailyDigestQueueRequired
	}

	digestDate := now.In(shanghaiLocation()).Format("2006-01-02")

	if err := s.queue.EnqueueDailyDigest(ctx, digestDate); err != nil {
		if errors.Is(err, ErrDailyDigestAlreadyQueued) {
			if s.metrics != nil {
				s.metrics.IncDailyDigestSkipped()
			}
			return JobTriggerResult{DigestDate: digestDate, Status: "skipped"}, nil
		}
		return JobTriggerResult{}, err
	}

	if s.metrics != nil {
		s.metrics.IncDailyDigestTriggered()
	}

	return JobTriggerResult{DigestDate: digestDate, Status: "accepted"}, nil
}

func IsDailyDigestAlreadyQueued(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrDailyDigestAlreadyQueued)
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
