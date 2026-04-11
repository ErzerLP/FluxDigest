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

var ErrDigestPendingState = errors.New("digest state is pending confirmation")

// ProcessingRunner 定义待入选文章处理所需的最小能力。
type ProcessingRunner interface {
	ProcessPending(ctx context.Context, windowStart, windowEnd time.Time) ([]domaindigest.CandidateArticle, error)
}

// DigestRunner 定义日报生成所需的最小能力。
type DigestRunner interface {
	Generate(ctx context.Context, candidates []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error)
}

// DigestStore 定义日报占位、查询与发布回写所需的最小能力。
type DigestStore interface {
	GetRemoteURL(ctx context.Context, digestDate string) (string, bool, error)
	Reserve(ctx context.Context, digestDate string, digest daily_digest_workflow.Digest) (bool, error)
	MarkPublished(ctx context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult) error
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
	remoteURL, exists, err := s.digests.GetRemoteURL(ctx, digestDate)
	if err != nil {
		return RunResult{}, err
	}
	if exists {
		if remoteURL != "" {
			return RunResult{DigestDate: digestDate, RemoteURL: remoteURL}, nil
		}
		return RunResult{}, ErrDigestPendingState
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

	reserved, err := s.digests.Reserve(ctx, digestDate, digest)
	if err != nil {
		return RunResult{}, err
	}
	if !reserved {
		remoteURL, exists, err = s.digests.GetRemoteURL(ctx, digestDate)
		if err != nil {
			return RunResult{}, err
		}
		if exists && remoteURL != "" {
			return RunResult{DigestDate: digestDate, RemoteURL: remoteURL}, nil
		}
		return RunResult{}, ErrDigestPendingState
	}

	publishResult, err := s.publisher.PublishDigest(ctx, adapterpublisher.PublishDigestRequest{
		Title:           digest.Title,
		Subtitle:        digest.Subtitle,
		ContentMarkdown: digest.ContentMarkdown,
		ContentHTML:     digest.ContentHTML,
	})
	if err != nil {
		return RunResult{}, err
	}

	if err := s.digests.MarkPublished(ctx, digestDate, publishResult); err != nil {
		return RunResult{}, err
	}

	return RunResult{
		DigestDate: digestDate,
		RemoteURL:  publishResult.RemoteURL,
	}, nil
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
