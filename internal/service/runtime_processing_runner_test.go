package service_test

import (
	"context"
	"testing"
	"time"

	"rss-platform/internal/adapter/miniflux"
	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/dossier"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"

	"gorm.io/gorm"
)

type entryListerStub struct {
	entries []miniflux.Entry
}

func (s entryListerStub) ListEntries(_ context.Context, _, _ time.Time) ([]miniflux.Entry, error) {
	return s.entries, nil
}

type articleFinderStub struct {
	article         article.SourceArticle
	findByIDCalls   int
	findByMinifluxs int
}

func (s *articleFinderStub) FindByMinifluxEntryID(_ context.Context, _ int64) (article.SourceArticle, error) {
	s.findByMinifluxs++
	return s.article, nil
}

func (s *articleFinderStub) FindByID(_ context.Context, _ string) (article.SourceArticle, error) {
	s.findByIDCalls++
	return s.article, nil
}

type processingSvcStub struct {
	processed processing.ProcessedArticle
	calls     int
}

func (s *processingSvcStub) ProcessArticle(_ context.Context, _ article.SourceArticle) (processing.ProcessedArticle, error) {
	s.calls++
	return s.processed, nil
}

type processingStoreStub struct {
	record postgres.ProcessedArticleRecord
	err    error
	saved  []postgres.ProcessedArticleRecord
}

func (s *processingStoreStub) GetLatestByArticleID(_ context.Context, _ string) (postgres.ProcessedArticleRecord, error) {
	return s.record, s.err
}

func (s *processingStoreStub) Save(_ context.Context, input postgres.ProcessedArticleRecord) error {
	s.saved = append(s.saved, input)
	return nil
}

type dossierMaterializerStub struct {
	dossier dossier.ArticleDossier
	inputs  []service.MaterializeDossierInput
}

func (s *dossierMaterializerStub) Materialize(_ context.Context, input service.MaterializeDossierInput) (dossier.ArticleDossier, error) {
	s.inputs = append(s.inputs, input)
	return s.dossier, nil
}

func TestRuntimeProcessingRunnerReturnsDossierDerivedCandidates(t *testing.T) {
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", TopicCategory: "AI", ImportanceScore: 0.8, RecommendationReason: "值得重点跟进", ReadingValue: "高", PriorityLevel: "high"}}
	finder := &articleFinderStub{article: article.SourceArticle{ID: "art-1", Title: "Model News"}}
	processor := &processingSvcStub{processed: processing.ProcessedArticle{Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"}, Analysis: processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8}}}
	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		finder,
		processor,
		store,
		materializer,
		service.RuntimePromptVersions{Translation: 6, Analysis: 6, Dossier: 6, LLM: 4},
	)

	items, err := runner.ProcessPending(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DossierID != "dos-1" {
		t.Fatalf("unexpected candidates %+v", items)
	}
}

func TestRuntimeProcessingRunnerAssignsProcessingIDBeforeMaterialize(t *testing.T) {
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1"}}
	finder := &articleFinderStub{article: article.SourceArticle{ID: "art-1", Title: "Model News"}}
	processor := &processingSvcStub{processed: processing.ProcessedArticle{Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"}, Analysis: processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8}}}
	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		finder,
		processor,
		store,
		materializer,
		service.RuntimePromptVersions{Translation: 6, Analysis: 6, Dossier: 6, LLM: 4},
	)

	if _, err := runner.ProcessPending(context.Background(), time.Now().Add(-time.Hour), time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected one processing save, got %d", len(store.saved))
	}
	if store.saved[0].ID == "" {
		t.Fatal("expected runner to assign processing id before save")
	}
	if len(materializer.inputs) != 1 {
		t.Fatalf("expected one dossier materialize call, got %d", len(materializer.inputs))
	}
	if materializer.inputs[0].ProcessingID != store.saved[0].ID {
		t.Fatalf("expected materialize processing id %q, got %q", store.saved[0].ID, materializer.inputs[0].ProcessingID)
	}
	if materializer.inputs[0].Processing.ID != store.saved[0].ID {
		t.Fatalf("expected materialize processing record id %q, got %q", store.saved[0].ID, materializer.inputs[0].Processing.ID)
	}
}

func TestRuntimeProcessingRunnerReprocessArticleRebuildsProcessingAndMaterializesDossier(t *testing.T) {
	store := &processingStoreStub{}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{ID: "dos-2", ArticleID: "art-1"}}
	finder := &articleFinderStub{article: article.SourceArticle{ID: "art-1", Title: "Model News"}}
	processor := &processingSvcStub{processed: processing.ProcessedArticle{
		Translation: processing.Translation{TitleTranslated: "重跑标题", SummaryTranslated: "重跑摘要", ContentTranslated: "重跑正文"},
		Analysis:    processing.Analysis{CoreSummary: "重跑核心总结", KeyPoints: []string{"r1"}, TopicCategory: "AI", ImportanceScore: 0.9},
	}}
	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{},
		finder,
		processor,
		store,
		materializer,
		service.RuntimePromptVersions{Translation: 6, Analysis: 6, Dossier: 6, LLM: 4},
	)

	if err := runner.ReprocessArticle(context.Background(), "art-1", true); err != nil {
		t.Fatal(err)
	}

	if finder.findByIDCalls != 1 {
		t.Fatalf("want find by id called once got %d", finder.findByIDCalls)
	}
	if processor.calls != 1 {
		t.Fatalf("want processor called once got %d", processor.calls)
	}
	if len(store.saved) != 1 {
		t.Fatalf("want one save got %d", len(store.saved))
	}
	if store.saved[0].ArticleID != "art-1" {
		t.Fatalf("want saved article art-1 got %s", store.saved[0].ArticleID)
	}
	if len(materializer.inputs) != 1 {
		t.Fatalf("want one materialize call got %d", len(materializer.inputs))
	}
	if materializer.inputs[0].ArticleID != "art-1" {
		t.Fatalf("want materialize article art-1 got %s", materializer.inputs[0].ArticleID)
	}
	if materializer.inputs[0].ProcessingID == "" {
		t.Fatal("want non-empty processing id for reprocess")
	}
}
