package service

import (
	"context"
	"errors"
	"time"
)

var errDailyDigestQueueRequired = errors.New("daily digest queue is required")
var errDailyDigestForceQueueRequired = errors.New("daily digest force queue is required")
var errArticleReprocessQueueRequired = errors.New("article reprocess queue is required")
var ErrDailyDigestAlreadyQueued = errors.New("daily digest already queued")
var ErrArticleReprocessAlreadyQueued = errors.New("article reprocess already queued")

// JobTriggerResult 表示手动或计划触发日报后的真实受理状态。
type JobTriggerResult struct {
	DigestDate string
	ArticleID  string
	Status     string
}

// DailyDigestTriggerOptions 描述日报触发选项。
type DailyDigestTriggerOptions struct {
	Force bool
}

// DailyDigestQueue 定义日报任务入队所需的最小能力。
type DailyDigestQueue interface {
	EnqueueDailyDigest(ctx context.Context, digestDate string) error
}

// DailyDigestForceQueue 定义支持 force 触发日报任务入队的能力。
type DailyDigestForceQueue interface {
	EnqueueDailyDigestWithOptions(ctx context.Context, digestDate string, opts DailyDigestTriggerOptions) error
}

// ArticleReprocessQueue 定义单篇重跑任务入队所需的最小能力。
type ArticleReprocessQueue interface {
	EnqueueArticleReprocess(ctx context.Context, articleID string, force bool) error
}

// JobMetrics 定义作业指标上报所需的最小能力。
type JobMetrics interface {
	IncDailyDigestTriggered()
	IncDailyDigestSkipped()
}

// JobService 负责按 digestDate 做最小幂等触发。
type JobService struct {
	queue        DailyDigestQueue
	articleQueue ArticleReprocessQueue
	metrics      JobMetrics
}

// NewJobService 创建 JobService。
func NewJobService(queue DailyDigestQueue, articleQueue ArticleReprocessQueue, metrics ...JobMetrics) *JobService {
	svc := &JobService{queue: queue, articleQueue: articleQueue}
	if len(metrics) > 0 {
		svc.metrics = metrics[0]
	}
	return svc
}

// TriggerDailyDigest 按上海日历日触发日报任务，重复日期会被跳过。
func (s *JobService) TriggerDailyDigest(ctx context.Context, now time.Time) (JobTriggerResult, error) {
	return s.TriggerDailyDigestWithOptions(ctx, now, DailyDigestTriggerOptions{})
}

// TriggerDailyDigestWithOptions 按上海日历日触发日报任务，并支持 force 选项。
func (s *JobService) TriggerDailyDigestWithOptions(ctx context.Context, now time.Time, opts DailyDigestTriggerOptions) (JobTriggerResult, error) {
	if s.queue == nil {
		return JobTriggerResult{}, errDailyDigestQueueRequired
	}

	digestDate := now.In(shanghaiLocation()).Format("2006-01-02")
	var err error
	if opts.Force {
		forceQueue, ok := s.queue.(DailyDigestForceQueue)
		if !ok {
			return JobTriggerResult{}, errDailyDigestForceQueueRequired
		}
		err = forceQueue.EnqueueDailyDigestWithOptions(ctx, digestDate, opts)
	} else {
		err = s.queue.EnqueueDailyDigest(ctx, digestDate)
	}
	if err != nil {
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

// TriggerArticleReprocess 触发单篇文章重跑任务。
func (s *JobService) TriggerArticleReprocess(ctx context.Context, articleID string, force bool) (JobTriggerResult, error) {
	if s.articleQueue == nil {
		return JobTriggerResult{}, errArticleReprocessQueueRequired
	}

	if err := s.articleQueue.EnqueueArticleReprocess(ctx, articleID, force); err != nil {
		if errors.Is(err, ErrArticleReprocessAlreadyQueued) {
			return JobTriggerResult{ArticleID: articleID, Status: "skipped"}, nil
		}
		return JobTriggerResult{}, err
	}

	return JobTriggerResult{ArticleID: articleID, Status: "accepted"}, nil
}

func IsArticleReprocessAlreadyQueued(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrArticleReprocessAlreadyQueued)
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
