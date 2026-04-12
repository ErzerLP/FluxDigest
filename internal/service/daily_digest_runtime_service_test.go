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
	state              string
	remoteURL          string
	remoteID           string
	publishError       string
	beginCalls         int
	markPublishedCalls int
	markFailedCalls    int
	markRecoveryCalls  int
	lastDigestDate     string
	lastDigest         daily_digest_workflow.Digest
	lastPublish        adapterpublisher.PublishDigestResult
	markPublishedErrs  []error
}

func (s *digestRepoStub) GetState(_ context.Context, digestDate string) (string, string, bool, error) {
	s.lastDigestDate = digestDate
	if s.state == "" {
		return "", "", false, nil
	}
	return s.state, s.remoteURL, true, nil
}

func (s *digestRepoStub) BeginPublish(_ context.Context, digestDate string, digest daily_digest_workflow.Digest) (bool, error) {
	s.beginCalls++
	s.lastDigestDate = digestDate
	s.lastDigest = digest
	if s.state == "published" || s.state == "publishing" || s.state == "recovery_required" {
		return false, nil
	}
	s.state = "publishing"
	s.remoteURL = ""
	s.remoteID = ""
	s.publishError = ""
	return true, nil
}

func (s *digestRepoStub) MarkPublished(_ context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult) error {
	s.markPublishedCalls++
	s.lastDigestDate = digestDate
	s.lastPublish = publishResult
	if len(s.markPublishedErrs) > 0 {
		err := s.markPublishedErrs[0]
		s.markPublishedErrs = s.markPublishedErrs[1:]
		if err != nil {
			return err
		}
	}
	s.state = "published"
	s.remoteURL = publishResult.RemoteURL
	s.remoteID = publishResult.RemoteID
	s.publishError = ""
	return nil
}

func (s *digestRepoStub) MarkFailed(_ context.Context, digestDate string, publishError string) error {
	s.markFailedCalls++
	s.lastDigestDate = digestDate
	s.state = "failed"
	s.publishError = publishError
	return nil
}

func (s *digestRepoStub) MarkRecoveryRequired(_ context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult, publishError string) error {
	s.markRecoveryCalls++
	s.lastDigestDate = digestDate
	s.state = "recovery_required"
	s.remoteID = publishResult.RemoteID
	s.remoteURL = publishResult.RemoteURL
	s.lastPublish = publishResult
	s.publishError = publishError
	return nil
}

type publishStub struct {
	calls   int
	results []publishOutcome
}

type publishOutcome struct {
	result adapterpublisher.PublishDigestResult
	err    error
}

func (publishStub) Name() string { return "stub" }

func (s *publishStub) PublishDigest(_ context.Context, req adapterpublisher.PublishDigestRequest) (adapterpublisher.PublishDigestResult, error) {
	s.calls++
	if req.Title == "" {
		return adapterpublisher.PublishDigestResult{}, nil
	}
	if len(s.results) == 0 {
		return adapterpublisher.PublishDigestResult{RemoteID: "remote-1", RemoteURL: "https://example.com/digest/2026-04-11"}, nil
	}
	out := s.results[0]
	s.results = s.results[1:]
	return out.result, out.err
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
	if digests.state != "published" {
		t.Fatalf("want published state got %s", digests.state)
	}
}

func TestDailyDigestRuntimeServiceRetryablePublishFailureTransitionsToFailedAndAllowsRetry(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1", Title: "AI News", CoreSummary: "Summary"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}
	digests := &digestRepoStub{}
	publisher := &publishStub{results: []publishOutcome{
		{err: adapterpublisher.NewRetryablePublishError(errors.New("server 500"))},
		{result: adapterpublisher.PublishDigestResult{RemoteID: "remote-2", RemoteURL: "https://example.com/retry-success"}},
	}}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)
	now := time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600))

	_, err := svc.Run(context.Background(), "2026-04-11", now)
	if err == nil || err.Error() != "server 500" {
		t.Fatalf("want server 500 got %v", err)
	}
	if digests.state != "failed" {
		t.Fatalf("want failed state got %s", digests.state)
	}
	if publisher.calls != 1 {
		t.Fatalf("want 1 publish call got %d", publisher.calls)
	}

	out, err := svc.Run(context.Background(), "2026-04-11", now)
	if err != nil {
		t.Fatal(err)
	}
	if out.RemoteURL != "https://example.com/retry-success" {
		t.Fatalf("want retry success url got %s", out.RemoteURL)
	}
	if publisher.calls != 2 {
		t.Fatalf("want 2 publish calls got %d", publisher.calls)
	}
}

func TestDailyDigestRuntimeServiceAmbiguousPublishFailureTransitionsToRecoveryRequired(t *testing.T) {
	digests := &digestRepoStub{}
	publisher := &publishStub{results: []publishOutcome{{err: adapterpublisher.NewAmbiguousPublishError(errors.New("network timeout"))}}}
	svc := service.NewDailyDigestRuntimeService(&ingestionStub{}, &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}, &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}, digests, publisher)
	now := time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600))

	_, err := svc.Run(context.Background(), "2026-04-11", now)
	if !errors.Is(err, service.ErrDigestRecoveryRequired) {
		t.Fatalf("want ErrDigestRecoveryRequired got %v", err)
	}
	if digests.state != "recovery_required" {
		t.Fatalf("want recovery_required got %s", digests.state)
	}
	if publisher.calls != 1 {
		t.Fatalf("want 1 publish call got %d", publisher.calls)
	}

	_, err = svc.Run(context.Background(), "2026-04-11", now)
	if !errors.Is(err, service.ErrDigestRecoveryRequired) {
		t.Fatalf("want ErrDigestRecoveryRequired on retry got %v", err)
	}
	if publisher.calls != 1 {
		t.Fatalf("expected no republish on retry, got %d calls", publisher.calls)
	}
}

func TestDailyDigestRuntimeServiceForceRunBypassesPublishedShortcut(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}
	digests := &digestRepoStub{state: "published", remoteURL: "https://example.com/original"}
	publisher := &publishStub{results: []publishOutcome{{result: adapterpublisher.PublishDigestResult{RemoteID: "remote-force", RemoteURL: "https://example.com/force"}}}}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)

	out, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)), service.RunOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.RemoteURL != "https://example.com/force" {
		t.Fatalf("want forced publish url got %s", out.RemoteURL)
	}
	if ingestion.calls != 1 {
		t.Fatalf("want ingestion called once got %d", ingestion.calls)
	}
	if processing.calls != 1 {
		t.Fatalf("want processing called once got %d", processing.calls)
	}
	if digestWorkflow.calls != 1 {
		t.Fatalf("want digest workflow called once got %d", digestWorkflow.calls)
	}
	if publisher.calls != 1 {
		t.Fatalf("want publisher called once got %d", publisher.calls)
	}
	if digests.state != "published" {
		t.Fatalf("want state published got %s", digests.state)
	}
}

func TestDailyDigestRuntimeServiceForceRunBypassesFailedStateShortcut(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}
	digests := &digestRepoStub{state: "failed", remoteURL: "https://example.com/failed-old"}
	publisher := &publishStub{results: []publishOutcome{{result: adapterpublisher.PublishDigestResult{RemoteID: "remote-force-failed", RemoteURL: "https://example.com/force-failed"}}}}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)

	out, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)), service.RunOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.RemoteURL != "https://example.com/force-failed" {
		t.Fatalf("want forced publish url got %s", out.RemoteURL)
	}
	if ingestion.calls != 1 {
		t.Fatalf("want ingestion called once got %d", ingestion.calls)
	}
	if processing.calls != 1 {
		t.Fatalf("want processing called once got %d", processing.calls)
	}
	if digestWorkflow.calls != 1 {
		t.Fatalf("want digest workflow called once got %d", digestWorkflow.calls)
	}
	if publisher.calls != 1 {
		t.Fatalf("want publisher called once got %d", publisher.calls)
	}
	if digests.markFailedCalls != 0 {
		t.Fatalf("failed state force rerun should start directly, got markFailedCalls=%d", digests.markFailedCalls)
	}
	if digests.state != "published" {
		t.Fatalf("want state published got %s", digests.state)
	}
}

func TestDailyDigestRuntimeServiceForceRunBypassesPublishingStateShortcut(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}
	digests := &digestRepoStub{state: "publishing", remoteURL: "https://example.com/publishing-old"}
	publisher := &publishStub{results: []publishOutcome{{result: adapterpublisher.PublishDigestResult{RemoteID: "remote-force-publishing", RemoteURL: "https://example.com/force-publishing"}}}}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)

	out, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)), service.RunOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.RemoteURL != "https://example.com/force-publishing" {
		t.Fatalf("want forced publish url got %s", out.RemoteURL)
	}
	if ingestion.calls != 1 {
		t.Fatalf("want ingestion called once got %d", ingestion.calls)
	}
	if processing.calls != 1 {
		t.Fatalf("want processing called once got %d", processing.calls)
	}
	if digestWorkflow.calls != 1 {
		t.Fatalf("want digest workflow called once got %d", digestWorkflow.calls)
	}
	if publisher.calls != 1 {
		t.Fatalf("want publisher called once got %d", publisher.calls)
	}
	if digests.markFailedCalls != 1 {
		t.Fatalf("publishing state force rerun should preempt once, got markFailedCalls=%d", digests.markFailedCalls)
	}
	if digests.beginCalls != 2 {
		t.Fatalf("publishing state force rerun should retry begin publish once, got beginCalls=%d", digests.beginCalls)
	}
	if digests.state != "published" {
		t.Fatalf("want state published got %s", digests.state)
	}
}

func TestDailyDigestRuntimeServiceForceRunStillBlocksRecoveryRequired(t *testing.T) {
	ingestion := &ingestionStub{}
	processing := &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}
	digestWorkflow := &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}
	digests := &digestRepoStub{state: "recovery_required"}
	publisher := &publishStub{}
	svc := service.NewDailyDigestRuntimeService(ingestion, processing, digestWorkflow, digests, publisher)

	_, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)), service.RunOptions{Force: true})
	if !errors.Is(err, service.ErrDigestRecoveryRequired) {
		t.Fatalf("want ErrDigestRecoveryRequired got %v", err)
	}
	if ingestion.calls != 0 {
		t.Fatalf("want no ingestion calls got %d", ingestion.calls)
	}
	if processing.calls != 0 {
		t.Fatalf("want no processing calls got %d", processing.calls)
	}
	if publisher.calls != 0 {
		t.Fatalf("want no publish calls got %d", publisher.calls)
	}
}

func TestDailyDigestRuntimeServiceRetriesMarkPublishedBeforeRecoveryRequired(t *testing.T) {
	digests := &digestRepoStub{markPublishedErrs: []error{errors.New("db timeout"), errors.New("db timeout"), nil}}
	publisher := &publishStub{}
	svc := service.NewDailyDigestRuntimeService(&ingestionStub{}, &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}, &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}, digests, publisher)

	out, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if out.RemoteURL == "" {
		t.Fatal("expected remote url")
	}
	if digests.markPublishedCalls != 3 {
		t.Fatalf("want 3 mark published attempts got %d", digests.markPublishedCalls)
	}
	if digests.state != "published" {
		t.Fatalf("want published state got %s", digests.state)
	}
	if digests.markRecoveryCalls != 0 {
		t.Fatalf("want no recovery_required mark got %d", digests.markRecoveryCalls)
	}
}

func TestDailyDigestRuntimeServiceMarkPublishedFailureTransitionsToRecoveryRequiredWithRemoteInfo(t *testing.T) {
	digests := &digestRepoStub{markPublishedErrs: []error{errors.New("db timeout"), errors.New("db timeout"), errors.New("db timeout")}}
	publisher := &publishStub{results: []publishOutcome{{result: adapterpublisher.PublishDigestResult{RemoteID: "remote-1", RemoteURL: "https://example.com/published"}}}}
	svc := service.NewDailyDigestRuntimeService(&ingestionStub{}, &processingWorkflowStub{candidates: []domaindigest.CandidateArticle{{ID: "art-1"}}}, &digestWorkflowStub{digest: daily_digest_workflow.Digest{Title: "日报", ContentMarkdown: "# 内容", ContentHTML: "<h1>内容</h1>"}}, digests, publisher)

	_, err := svc.Run(context.Background(), "2026-04-11", time.Date(2026, 4, 12, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if !errors.Is(err, service.ErrDigestRecoveryRequired) {
		t.Fatalf("want ErrDigestRecoveryRequired got %v", err)
	}
	if digests.state != "recovery_required" {
		t.Fatalf("want recovery_required state got %s", digests.state)
	}
	if digests.remoteID != "remote-1" {
		t.Fatalf("want remote-1 got %s", digests.remoteID)
	}
	if digests.remoteURL != "https://example.com/published" {
		t.Fatalf("want published url got %s", digests.remoteURL)
	}
	if publisher.calls != 1 {
		t.Fatalf("want 1 publish call got %d", publisher.calls)
	}
}
