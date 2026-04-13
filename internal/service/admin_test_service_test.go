package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"rss-platform/internal/service"
)

type llmCheckerStub struct {
	latency time.Duration
	err     error
	drafts  []service.LLMTestDraft
}

func (s *llmCheckerStub) Check(_ context.Context, draft service.LLMTestDraft) (time.Duration, error) {
	s.drafts = append(s.drafts, draft)
	return s.latency, s.err
}

type connectivityCheckerStub struct {
	latency time.Duration
	err     error
	checks  int
}

func (s *connectivityCheckerStub) Check(_ context.Context) (time.Duration, error) {
	s.checks++
	return s.latency, s.err
}

type jobRunWriterStub struct {
	created []service.JobRunRecord
	err     error
}

func (s *jobRunWriterStub) Create(_ context.Context, record service.JobRunRecord) error {
	s.created = append(s.created, record)
	return s.err
}

var errStub = errors.New("persist failed")

func TestAdminTestServiceRecordsLLMResult(t *testing.T) {
	checker := &llmCheckerStub{latency: 850 * time.Millisecond}
	repo := &jobRunWriterStub{}
	svc := service.NewAdminTestService(checker, &connectivityCheckerStub{}, &connectivityCheckerStub{}, repo)

	result, err := svc.TestLLM(context.Background(), service.LLMTestDraft{BaseURL: "https://llm.local/v1", Model: "gpt-4.1-mini", APIKey: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("want ok got %q", result.Status)
	}
	if len(repo.created) != 1 || repo.created[0].JobType != "llm_test" {
		t.Fatalf("unexpected records %#v", repo.created)
	}
	if len(checker.drafts) != 1 || checker.drafts[0].TimeoutMS != 30000 {
		t.Fatalf("expected default timeout_ms=30000 got %#v", checker.drafts)
	}
}

func TestAdminTestServiceReturnsErrorWhenPersistFails(t *testing.T) {
	checker := &llmCheckerStub{latency: 10 * time.Millisecond}
	repo := &jobRunWriterStub{err: errStub}
	svc := service.NewAdminTestService(checker, &connectivityCheckerStub{}, &connectivityCheckerStub{}, repo)

	_, err := svc.TestLLM(context.Background(), service.LLMTestDraft{BaseURL: "https://llm.local/v1", Model: "gpt-4.1-mini", APIKey: "token"})
	if err == nil {
		t.Fatal("expected error when persist fails")
	}
}

func TestAdminTestServicePassesThroughTimeoutMS(t *testing.T) {
	checker := &llmCheckerStub{latency: 5 * time.Millisecond}
	repo := &jobRunWriterStub{}
	svc := service.NewAdminTestService(checker, &connectivityCheckerStub{}, &connectivityCheckerStub{}, repo)

	_, err := svc.TestLLM(context.Background(), service.LLMTestDraft{
		BaseURL:   "https://llm.local/v1",
		Model:     "gpt-4.1-mini",
		APIKey:    "token",
		TimeoutMS: 45000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(checker.drafts) != 1 || checker.drafts[0].TimeoutMS != 45000 {
		t.Fatalf("expected timeout_ms pass-through got %#v", checker.drafts)
	}
}

func TestAdminTestServiceCapsOversizedTimeoutMS(t *testing.T) {
	checker := &llmCheckerStub{latency: 5 * time.Millisecond}
	repo := &jobRunWriterStub{}
	svc := service.NewAdminTestService(checker, &connectivityCheckerStub{}, &connectivityCheckerStub{}, repo)

	_, err := svc.TestLLM(context.Background(), service.LLMTestDraft{
		BaseURL:   "https://llm.local/v1",
		Model:     "gpt-4.1-mini",
		APIKey:    "token",
		TimeoutMS: 3_000_000_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(checker.drafts) != 1 || checker.drafts[0].TimeoutMS != 2_147_483_647 {
		t.Fatalf("expected capped timeout_ms got %#v", checker.drafts)
	}
}

func TestAdminTestServiceRecordsMinifluxResult(t *testing.T) {
	checker := &connectivityCheckerStub{latency: 320 * time.Millisecond}
	repo := &jobRunWriterStub{}
	svc := service.NewAdminTestService(&llmCheckerStub{}, checker, &connectivityCheckerStub{}, repo)

	result, err := svc.TestMiniflux(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("want ok got %q", result.Status)
	}
	if checker.checks != 1 {
		t.Fatalf("want checker called once got %d", checker.checks)
	}
	if len(repo.created) != 1 || repo.created[0].JobType != "miniflux_test" {
		t.Fatalf("unexpected records %#v", repo.created)
	}
}

func TestAdminTestServiceRecordsPublishFailure(t *testing.T) {
	checker := &connectivityCheckerStub{latency: 120 * time.Millisecond, err: errors.New("publish unavailable")}
	repo := &jobRunWriterStub{}
	svc := service.NewAdminTestService(&llmCheckerStub{}, &connectivityCheckerStub{}, checker, repo)

	result, err := svc.TestPublish(context.Background())
	if err == nil {
		t.Fatal("expected publish test error")
	}
	if result.Status != "error" {
		t.Fatalf("want error got %q", result.Status)
	}
	if len(repo.created) != 1 || repo.created[0].JobType != "publish_test" || repo.created[0].Status != "error" {
		t.Fatalf("unexpected records %#v", repo.created)
	}
}
