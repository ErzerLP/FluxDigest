package main

import (
	"context"
	"os"
	"testing"
	"time"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/repository/postgres"

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

	translationTemplate, analysisTemplate, err := loadDefaultPromptTemplates()
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

	runner := &runtimeProcessingRunner{
		client: entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articles: articleFinderStub{article: article.SourceArticle{
			ID:              "art-1",
			MinifluxEntryID: 101,
			Title:           "Original",
		}},
		processing: processor,
		results:    store,
	}

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

	runner := &runtimeProcessingRunner{
		client: entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articles: articleFinderStub{article: article.SourceArticle{
			ID:              "art-1",
			MinifluxEntryID: 101,
			Title:           "Original",
		}},
		processing: processor,
		results:    store,
	}

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
	if candidates[0] != (domaindigest.CandidateArticle{ID: "art-1", Title: "新标题", CoreSummary: "新核心总结"}) {
		t.Fatalf("unexpected candidate %+v", candidates[0])
	}
}
