package service

import (
	"context"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

// ProcessingRunner 定义待入选文章处理所需的最小能力。
type ProcessingRunner interface {
	ProcessPending(ctx context.Context, since time.Time) ([]domaindigest.CandidateArticle, error)
}

// DigestRunner 定义日报生成所需的最小能力。
type DigestRunner interface {
	Generate(ctx context.Context, candidates []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error)
}

// DigestWriter 定义日报结果持久化所需的最小能力。
type DigestWriter interface {
	Save(ctx context.Context, runAt time.Time, digest daily_digest_workflow.Digest, publishResult adapterpublisher.PublishDigestResult) error
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
	digests    DigestWriter
	publisher  adapterpublisher.Publisher
}

// NewDailyDigestRuntimeService 创建运行期日报编排服务。
func NewDailyDigestRuntimeService(
	ingestion ArticleIngestionRunner,
	processing ProcessingRunner,
	digest DigestRunner,
	digests DigestWriter,
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
func (s *DailyDigestRuntimeService) Run(ctx context.Context, now time.Time) (RunResult, error) {
	runAt := now.In(shanghaiLocation())
	since := time.Date(runAt.Year(), runAt.Month(), runAt.Day(), 0, 0, 0, 0, runAt.Location())

	if err := s.ingestion.FetchAndPersist(ctx, since); err != nil {
		return RunResult{}, err
	}

	candidates, err := s.processing.ProcessPending(ctx, since)
	if err != nil {
		return RunResult{}, err
	}

	digest, err := s.digest.Generate(ctx, candidates)
	if err != nil {
		return RunResult{}, err
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

	if err := s.digests.Save(ctx, runAt, digest, publishResult); err != nil {
		return RunResult{}, err
	}

	return RunResult{
		DigestDate: runAt.Format("2006-01-02"),
		RemoteURL:  publishResult.RemoteURL,
	}, nil
}
