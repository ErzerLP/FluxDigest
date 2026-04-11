package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

var (
	ErrDigestPublishInProgress = errors.New("digest publish is in progress")
	ErrDigestRecoveryRequired  = errors.New("digest publish requires recovery")
)

const (
	digestStatePublishing       = "publishing"
	digestStatePublished        = "published"
	digestStateFailed           = "failed"
	digestStateRecoveryRequired = "recovery_required"
	markPublishedRetryLimit     = 3
)

// ProcessingRunner 定义待入选文章处理所需的最小能力。
type ProcessingRunner interface {
	ProcessPending(ctx context.Context, windowStart, windowEnd time.Time) ([]domaindigest.CandidateArticle, error)
}

// DigestRunner 定义日报生成所需的最小能力。
type DigestRunner interface {
	Generate(ctx context.Context, candidates []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error)
}

// DigestStore 定义日报状态机所需的最小能力。
type DigestStore interface {
	GetState(ctx context.Context, digestDate string) (state string, remoteURL string, found bool, err error)
	BeginPublish(ctx context.Context, digestDate string, digest daily_digest_workflow.Digest) (bool, error)
	MarkPublished(ctx context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult) error
	MarkFailed(ctx context.Context, digestDate string, publishError string) error
	MarkRecoveryRequired(ctx context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult, publishError string) error
}

// RunResult 表示一次日报运行的关键输出。
type RunResult struct {
	DigestDate string
	RemoteURL  string
}

// DailyDigestRuntimeService 负责编排抓取、处理、日报生成、发布与持久化。
type DailyDigestRuntimeService struct {
	ingestion  ArticleIngestionRunner
	processing ProcessingRunner
	digest     DigestRunner
	digests    DigestStore
	publisher  adapterpublisher.Publisher
}

// NewDailyDigestRuntimeService 创建运行期日报编排服务。
func NewDailyDigestRuntimeService(
	ingestion ArticleIngestionRunner,
	processing ProcessingRunner,
	digest DigestRunner,
	digests DigestStore,
	publisher adapterpublisher.Publisher,
) *DailyDigestRuntimeService {
	return &DailyDigestRuntimeService{
		ingestion:  ingestion,
		processing: processing,
		digest:     digest,
		digests:    digests,
		publisher:  publisher,
	}
}

// Run 执行一次日报运行链路，并返回关键结果。
func (s *DailyDigestRuntimeService) Run(ctx context.Context, digestDate string, now time.Time) (RunResult, error) {
	state, remoteURL, found, err := s.digests.GetState(ctx, digestDate)
	if err != nil {
		return RunResult{}, err
	}
	if found {
		if out, handled, err := s.handleExistingState(digestDate, state, remoteURL); handled {
			return out, err
		}
	}

	windowStart, windowEnd, err := digestWindow(digestDate, now)
	if err != nil {
		return RunResult{}, err
	}

	if err := s.ingestion.FetchAndPersist(ctx, windowStart, windowEnd); err != nil {
		return RunResult{}, err
	}

	candidates, err := s.processing.ProcessPending(ctx, windowStart, windowEnd)
	if err != nil {
		return RunResult{}, err
	}

	digest, err := s.digest.Generate(ctx, candidates)
	if err != nil {
		return RunResult{}, err
	}

	started, err := s.digests.BeginPublish(ctx, digestDate, digest)
	if err != nil {
		return RunResult{}, err
	}
	if !started {
		state, remoteURL, found, err = s.digests.GetState(ctx, digestDate)
		if err != nil {
			return RunResult{}, err
		}
		if out, handled, err := s.handleExistingState(digestDate, state, remoteURL); handled {
			return out, err
		}
		return RunResult{}, ErrDigestPublishInProgress
	}

	publishResult, err := s.publisher.PublishDigest(ctx, adapterpublisher.PublishDigestRequest{
		Title:           digest.Title,
		Subtitle:        digest.Subtitle,
		ContentMarkdown: digest.ContentMarkdown,
		ContentHTML:     digest.ContentHTML,
	})
	if err != nil {
		return RunResult{}, s.handlePublishError(ctx, digestDate, err)
	}

	if err := s.markPublishedWithRetry(ctx, digestDate, publishResult); err != nil {
		markErr := s.digests.MarkRecoveryRequired(ctx, digestDate, publishResult, err.Error())
		if markErr != nil {
			return RunResult{}, errors.Join(ErrDigestRecoveryRequired, err, markErr)
		}
		return RunResult{}, errors.Join(ErrDigestRecoveryRequired, err)
	}

	return RunResult{DigestDate: digestDate, RemoteURL: publishResult.RemoteURL}, nil
}

func (s *DailyDigestRuntimeService) handleExistingState(digestDate, state, remoteURL string) (RunResult, bool, error) {
	switch state {
	case digestStatePublished:
		return RunResult{DigestDate: digestDate, RemoteURL: remoteURL}, true, nil
	case digestStatePublishing:
		return RunResult{}, true, ErrDigestPublishInProgress
	case digestStateRecoveryRequired:
		return RunResult{}, true, ErrDigestRecoveryRequired
	default:
		return RunResult{}, false, nil
	}
}

func (s *DailyDigestRuntimeService) handlePublishError(ctx context.Context, digestDate string, publishErr error) error {
	if adapterpublisher.IsAmbiguousPublishError(publishErr) {
		markErr := s.digests.MarkRecoveryRequired(ctx, digestDate, adapterpublisher.PublishDigestResult{}, publishErr.Error())
		if markErr != nil {
			return errors.Join(ErrDigestRecoveryRequired, publishErr, markErr)
		}
		return errors.Join(ErrDigestRecoveryRequired, publishErr)
	}

	markErr := s.digests.MarkFailed(ctx, digestDate, publishErr.Error())
	if markErr != nil {
		return errors.Join(publishErr, markErr)
	}
	return publishErr
}

func (s *DailyDigestRuntimeService) markPublishedWithRetry(ctx context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult) error {
	var lastErr error
	for attempt := 0; attempt < markPublishedRetryLimit; attempt++ {
		if err := s.digests.MarkPublished(ctx, digestDate, publishResult); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func digestWindow(digestDate string, now time.Time) (time.Time, time.Time, error) {
	windowStart, err := time.ParseInLocation("2006-01-02", digestDate, shanghaiLocation())
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse digest date %s: %w", digestDate, err)
	}

	windowEnd := now.In(shanghaiLocation())
	dayEnd := windowStart.Add(24 * time.Hour)
	if windowEnd.After(dayEnd) {
		windowEnd = dayEnd
	}
	if windowEnd.Before(windowStart) {
		windowEnd = windowStart
	}

	return windowStart, windowEnd, nil
}
