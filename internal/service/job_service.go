package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errDailyDigestQueueRequired = errors.New("daily digest queue is required")

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
	queue      DailyDigestQueue
	metrics    JobMetrics
	mu         sync.Mutex
	lastDigest string
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
func (s *JobService) TriggerDailyDigest(ctx context.Context, now time.Time) error {
	if s.queue == nil {
		return errDailyDigestQueueRequired
	}

	digestDate := now.In(shanghaiLocation()).Format("2006-01-02")

	s.mu.Lock()
	if s.lastDigest == digestDate {
		s.mu.Unlock()
		if s.metrics != nil {
			s.metrics.IncDailyDigestSkipped()
		}
		return nil
	}
	s.lastDigest = digestDate
	s.mu.Unlock()

	if err := s.queue.EnqueueDailyDigest(ctx, digestDate); err != nil {
		s.mu.Lock()
		if s.lastDigest == digestDate {
			s.lastDigest = ""
		}
		s.mu.Unlock()
		return err
	}

	if s.metrics != nil {
		s.metrics.IncDailyDigestTriggered()
	}

	return nil
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
