package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"rss-platform/internal/domain/article"
	domaindossier "rss-platform/internal/domain/dossier"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"

	"gorm.io/gorm"

	"rss-platform/internal/adapter/miniflux"
	domaindigest "rss-platform/internal/domain/digest"
)

type entryListerStub struct {
	entries []miniflux.Entry
}

func (s entryListerStub) ListEntries(_ context.Context, _, _ time.Time) ([]miniflux.Entry, error) {
	return s.entries, nil
}

type articleFinderStub struct {
	article article.SourceArticle
}

func (s articleFinderStub) FindByMinifluxEntryID(_ context.Context, _ int64) (article.SourceArticle, error) {
	return s.article, nil
}

func (s articleFinderStub) FindByID(_ context.Context, _ string) (article.SourceArticle, error) {
	return s.article, nil
}

type processingServiceStub struct {
	called int
	result processing.ProcessedArticle
}

func (s *processingServiceStub) ProcessArticle(_ context.Context, _ article.SourceArticle) (processing.ProcessedArticle, error) {
	s.called++
	return s.result, nil
}

type processingStoreStub struct {
	record       postgres.ProcessedArticleRecord
	err          error
	getLatestHit int
	saveHit      int
}

func (s *processingStoreStub) GetLatestByArticleID(_ context.Context, _ string) (postgres.ProcessedArticleRecord, error) {
	s.getLatestHit++
	return s.record, s.err
}

func (s *processingStoreStub) Save(_ context.Context, _ postgres.ProcessedArticleRecord) error {
	s.saveHit++
	return nil
}

type dossierMaterializerStub struct {
	dossier domaindossier.ArticleDossier
}

func (s dossierMaterializerStub) Materialize(_ context.Context, _ service.MaterializeDossierInput) (domaindossier.ArticleDossier, error) {
	return s.dossier, nil
}

type chatGenerateResult struct {
	message *schema.Message
	err     error
}

type chatModelStub struct {
	results []chatGenerateResult
	calls   int
}

func (s *chatModelStub) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	s.calls++
	idx := s.calls - 1
	if idx >= len(s.results) {
		return nil, errors.New("unexpected generate call")
	}
	return s.results[idx].message, s.results[idx].err
}

func (s *chatModelStub) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}

func TestLoadDefaultPromptTemplatesIgnoresWorkingDirectory(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if chdirErr := os.Chdir(oldWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	}()

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	translationTemplate, analysisTemplate, _, _, err := loadDefaultPromptTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if translationTemplate == "" {
		t.Fatal("want translation template content")
	}
	if analysisTemplate == "" {
		t.Fatal("want analysis template content")
	}
}

func TestRuntimeProcessingRunnerReusesExistingProcessingResult(t *testing.T) {
	processor := &processingServiceStub{}
	store := &processingStoreStub{
		record: postgres.ProcessedArticleRecord{
			ArticleID:       "art-1",
			TitleTranslated: "已翻译标题",
			CoreSummary:     "已有核心总结",
			TopicCategory:   "AI",
			ImportanceScore: 0.9,
		},
	}

	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articleFinderStub{article: article.SourceArticle{
			ID:              "art-1",
			MinifluxEntryID: 101,
			Title:           "Original",
		}},
		processor,
		store,
		dossierMaterializerStub{dossier: domaindossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "已翻译标题", CoreSummary: "已有核心总结", TopicCategory: "AI", ImportanceScore: 0.9}},
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)

	candidates, err := runner.ProcessPending(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("want 1 candidate got %d", len(candidates))
	}
	if got := candidates[0].Title; got != "已翻译标题" {
		t.Fatalf("want reused translated title got %s", got)
	}
	if got := candidates[0].DossierID; got != "dos-1" {
		t.Fatalf("want reused dossier id got %s", got)
	}
	if processor.called != 0 {
		t.Fatalf("want processor not called got %d", processor.called)
	}
	if store.saveHit != 0 {
		t.Fatalf("want save not called got %d", store.saveHit)
	}
	if store.getLatestHit != 1 {
		t.Fatalf("want get latest called once got %d", store.getLatestHit)
	}
}

func TestRuntimeProcessingRunnerProcessesAndSavesWhenNoExistingResult(t *testing.T) {
	processor := &processingServiceStub{
		result: processing.ProcessedArticle{
			Translation: processing.Translation{TitleTranslated: "新标题"},
			Analysis: processing.Analysis{
				CoreSummary:     "新核心总结",
				TopicCategory:   "AI",
				ImportanceScore: 0.8,
			},
		},
	}
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}

	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articleFinderStub{article: article.SourceArticle{
			ID:              "art-1",
			MinifluxEntryID: 101,
			Title:           "Original",
		}},
		processor,
		store,
		dossierMaterializerStub{dossier: domaindossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "新标题", CoreSummary: "新核心总结", TopicCategory: "AI", ImportanceScore: 0.8}},
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)

	candidates, err := runner.ProcessPending(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("want 1 candidate got %d", len(candidates))
	}
	if processor.called != 1 {
		t.Fatalf("want processor called once got %d", processor.called)
	}
	if store.saveHit != 1 {
		t.Fatalf("want save called once got %d", store.saveHit)
	}
	if candidates[0] != (domaindigest.CandidateArticle{ID: "art-1", DossierID: "dos-1", Title: "新标题", CoreSummary: "新核心总结", TopicCategory: "AI", ImportanceScore: 0.8}) {
		t.Fatalf("unexpected candidate %+v", candidates[0])
	}
}

func TestChatModelInvokerGenerateRetriesTransientErrorThenSucceeds(t *testing.T) {
	chat := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
			{message: &schema.Message{Content: "  ok  "}},
		},
	}
	invoker := chatModelInvoker{chat: chat}

	got, err := invoker.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("Generate() = %q, want %q", got, "ok")
	}
	if chat.calls != 2 {
		t.Fatalf("Generate() calls = %d, want 2", chat.calls)
	}
}

func TestChatModelInvokerGenerateNoRetryForNonTransientError(t *testing.T) {
	chat := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 400 invalid request")},
		},
	}
	invoker := chatModelInvoker{chat: chat}

	_, err := invoker.Generate(context.Background(), "prompt")
	if err == nil {
		t.Fatal("Generate() error = nil, want non-nil")
	}
	if chat.calls != 1 {
		t.Fatalf("Generate() calls = %d, want 1", chat.calls)
	}
}

func TestChatModelInvokerGenerateStopsRetryWhenContextCanceled(t *testing.T) {
	chat := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
			{message: &schema.Message{Content: "ok"}},
		},
	}
	invoker := chatModelInvoker{chat: chat}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := invoker.Generate(ctx, "prompt")
	if err == nil {
		t.Fatal("Generate() error = nil, want non-nil")
	}
	if chat.calls != 1 {
		t.Fatalf("Generate() calls = %d, want 1", chat.calls)
	}
}
