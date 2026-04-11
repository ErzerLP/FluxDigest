package service_test

import (
	"context"
	"testing"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/service"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

type ingestionStub struct {
	since time.Time
}

func (s *ingestionStub) FetchAndPersist(_ context.Context, since time.Time) error {
	s.since = since
	return nil
}

type processingWorkflowStub struct {
	candidates []domaindigest.CandidateArticle
}

func (s processingWorkflowStub) ProcessPending(_ context.Context, _ time.Time) ([]domaindigest.CandidateArticle, error) {
	return s.candidates, nil
}

type digestWorkflowStub struct {
	digest daily_digest_workflow.Digest
}

func (s digestWorkflowStub) Generate(_ context.Context, _ []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error) {
	return s.digest, nil
}

type digestRepoStub struct {
	saved         bool
	runAt         time.Time
	digest        daily_digest_workflow.Digest
	publishResult adapterpublisher.PublishDigestResult
}

func (s *digestRepoStub) Save(_ context.Context, runAt time.Time, digest daily_digest_workflow.Digest, publishResult adapterpublisher.PublishDigestResult) error {
	s.saved = true
	s.runAt = runAt
	s.digest = digest
	s.publishResult = publishResult
	return nil
}

type publishStub struct{}

func (publishStub) Name() string { return "stub" }

func (publishStub) PublishDigest(_ context.Context, req adapterpublisher.PublishDigestRequest) (adapterpublisher.PublishDigestResult, error) {
	if req.Title == "" {
		return adapterpublisher.PublishDigestResult{}, nil
	}
	return adapterpublisher.PublishDigestResult{RemoteID: "remote-1", RemoteURL: "https://example.com/digest/2026-04-11"}, nil
}

func TestDailyDigestRuntimeServiceRunsEndToEndAndPersistsDigest(t *testing.T) {
	ingestion := &ingestionStub{}
	digests := &digestRepoStub{}
	svc := service.NewDailyDigestRuntimeService(
		ingestion,
		processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1", Title: "AI News", CoreSummary: "Summary"}}},
		digestWorkflowStub{digest: daily_digest_workflow.Digest{
			Title:           "2026-04-11 AI 日报",
			Subtitle:        "聚焦模型与产品动态",
			ContentMarkdown: "# 内容",
			ContentHTML:     "<h1>内容</h1>",
		}},
		digests,
		publishStub{},
	)

	now := time.Date(2026, 4, 11, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))
	out, err := svc.Run(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if out.DigestDate != "2026-04-11" {
		t.Fatalf("want 2026-04-11 got %s", out.DigestDate)
	}
	if out.RemoteURL == "" {
		t.Fatal("expected remote url")
	}
	if !digests.saved {
		t.Fatal("expected digest repo save to be called")
	}
	wantSince := time.Date(2026, 4, 11, 0, 0, 0, 0, now.Location())
	if !ingestion.since.Equal(wantSince) {
		t.Fatalf("want since %s got %s", wantSince.Format(time.RFC3339), ingestion.since.Format(time.RFC3339))
	}
	if digests.digest.Title != "2026-04-11 AI 日报" {
		t.Fatalf("unexpected digest title: %s", digests.digest.Title)
	}
	if digests.publishResult.RemoteURL != out.RemoteURL {
		t.Fatalf("want saved remote url %s got %s", out.RemoteURL, digests.publishResult.RemoteURL)
	}
}
