package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/service"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

type ingestionStub struct {
	calls       int
	windowStart time.Time
	windowEnd   time.Time
}

func (s *ingestionStub) FetchAndPersist(_ context.Context, windowStart, windowEnd time.Time) error {
	s.calls++
	s.windowStart = windowStart
	s.windowEnd = windowEnd
	return nil
}

type processingWorkflowStub struct {
	calls       int
	windowStart time.Time
	windowEnd   time.Time
	candidates  []domaindigest.CandidateArticle
}

func (s *processingWorkflowStub) ProcessPending(_ context.Context, windowStart, windowEnd time.Time) ([]domaindigest.CandidateArticle, error) {
	s.calls++
	s.windowStart = windowStart
	s.windowEnd = windowEnd
	return s.candidates, nil
}

type digestWorkflowStub struct {
	calls  int
	digest daily_digest_workflow.Digest
}

func (s *digestWorkflowStub) Generate(_ context.Context, _ []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error) {
	s.calls++
	return s.digest, nil
}

type digestRepoStub struct {
	exists         bool
	remoteURL      string
	reserveCalls   int
	markCalls      int
	lastDigestDate string
	lastDigest     daily_digest_workflow.Digest
	lastPublish    adapterpublisher.PublishDigestResult
	markPublishErr error
	reserveReturn  bool
}

func (s *digestRepoStub) GetRemoteURL(_ context.Context, digestDate string) (string, bool, error) {
	s.lastDigestDate = digestDate
	return s.remoteURL, s.exists, nil
}

func (s *digestRepoStub) Reserve(_ context.Context, digestDate string, digest daily_digest_workflow.Digest) (bool, error) {
	s.reserveCalls++
	s.lastDigestDate = digestDate
	s.lastDigest = digest
	if s.exists {
		return false, nil
	}
	reserved := true
	if s.reserveReturn {
		reserved = s.reserveReturn
	}
	if reserved {
		s.exists = true
		s.remoteURL = ""
	}
	return reserved, nil
}

func (s *digestRepoStub) MarkPublished(_ context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult) error {
	s.markCalls++
	s.lastDigestDate = digestDate
	s.lastPublish = publishResult
	if s.markPublishErr != nil {
		return s.markPublishErr
	}
	s.exists = true
	s.remoteURL = publishResult.RemoteURL
	return nil
}

type publishStub struct {
	calls int
}

func (publishStub) Name() string { return "stub" }

func (s *publishStub) PublishDigest(_ context.Context, req adapterpublisher.PublishDigestRequest) (adapterpublisher.PublishDigestResult, error) {
	s.calls++
	if req.Title == "" {
		return adapterpublisher.PublishDigestResult{}, nil
	}
	return adapterpublisher.PublishDigestResult{RemoteID: "remote-1", RemoteURL: "https://example.com/digest/2026-04-11"}, nil
}

func TestDailyDigestRuntimeServiceRunsEndToEndAndPersistsDigest(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1", Title: "AI News", CoreSummary: "Summary"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{
		Title:           "2026-04-11 AI 日报",
		Subtitle:        "聚焦模型与产品动态",
		ContentMarkdown: "# 内容",
		ContentHTML:     "<h1>内容</h1>",
	}}
	digests := &digestRepoStub{}
	publisher := &publishStub{}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)

	now := time.Date(2026, 4, 12, 8, 30, 0, 0, time.FixedZone("CST", 8*3600))
	out, err := svc.Run(context.Background(), "2026-04-11", now)
	if err != nil {
		t.Fatal(err)
	}
	if out.DigestDate != "2026-04-11" {
		t.Fatalf("want 2026-04-11 got %s", out.DigestDate)
	}
	if out.RemoteURL == "" {
		t.Fatal("expected remote url")
	}
	wantStart := time.Date(2026, 4, 11, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	wantEnd := time.Date(2026, 4, 12, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	if !ingestion.windowStart.Equal(wantStart) || !ingestion.windowEnd.Equal(wantEnd) {
		t.Fatalf("unexpected ingestion window: [%s, %s)", ingestion.windowStart.Format(time.RFC3339), ingestion.windowEnd.Format(time.RFC3339))
	}
	if !processing.windowStart.Equal(wantStart) || !processing.windowEnd.Equal(wantEnd) {
		t.Fatalf("unexpected processing window: [%s, %s)", processing.windowStart.Format(time.RFC3339), processing.windowEnd.Format(time.RFC3339))
	}
	if publisher.calls != 1 {
		t.Fatalf("want 1 publish call got %d", publisher.calls)
	}
	if digests.reserveCalls != 1 || digests.markCalls != 1 {
		t.Fatalf("want reserve=1 mark=1 got reserve=%d mark=%d", digests.reserveCalls, digests.markCalls)
	}
}

func TestDailyDigestRuntimeServiceReturnsExistingPublishedDigestWithoutRepublishing(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{}
	digestWorkflow := &digestWorkflowStub{}
	digests := &digestRepoStub{exists: true, remoteURL: "https://example.com/existing"}
	publisher := &publishStub{}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)

	out, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if out.RemoteURL != "https://example.com/existing" {
		t.Fatalf("want existing remote url got %s", out.RemoteURL)
	}
	if publisher.calls != 0 {
		t.Fatalf("want 0 publish calls got %d", publisher.calls)
	}
	if ingestion.calls != 0 || processing.calls != 0 || digestWorkflow.calls != 0 {
		t.Fatalf("expected runtime to short-circuit before work: ingestion=%d processing=%d digest=%d", ingestion.calls, processing.calls, digestWorkflow.calls)
	}
}

func TestDailyDigestRuntimeServiceReturnsPendingErrorWithoutRepublishing(t *testing.T) {
	digests := &digestRepoStub{exists: true, remoteURL: ""}
	publisher := &publishStub{}
	svc := service.NewDailyDigestRuntimeService(&ingestionStub{}, &processingWorkflowStub{}, &digestWorkflowStub{}, digests, publisher)

	_, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if !errors.Is(err, service.ErrDigestPendingState) {
		t.Fatalf("want ErrDigestPendingState got %v", err)
	}
	if publisher.calls != 0 {
		t.Fatalf("want 0 publish calls got %d", publisher.calls)
	}
}

func TestDailyDigestRuntimeServiceDoesNotRepublishAfterMarkPublishedFails(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1", Title: "AI News", CoreSummary: "Summary"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}
	digests := &digestRepoStub{markPublishErr: errors.New("save failed")}
	publisher := &publishStub{}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)
	now := time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600))

	_, err := svc.Run(context.Background(), "2026-04-11", now)
	if err == nil || err.Error() != "save failed" {
		t.Fatalf("want save failed got %v", err)
	}
	if publisher.calls != 1 {
		t.Fatalf("want 1 publish call got %d", publisher.calls)
	}

	_, err = svc.Run(context.Background(), "2026-04-11", now)
	if !errors.Is(err, service.ErrDigestPendingState) {
		t.Fatalf("want ErrDigestPendingState on retry got %v", err)
	}
	if publisher.calls != 1 {
		t.Fatalf("expected no republish on retry, got %d calls", publisher.calls)
	}
}
