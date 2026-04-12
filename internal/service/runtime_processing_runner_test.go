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
	article article.SourceArticle
}

func (s articleFinderStub) FindByMinifluxEntryID(_ context.Context, _ int64) (article.SourceArticle, error) {
	return s.article, nil
}

type processingSvcStub struct {
	processed processing.ProcessedArticle
}

func (s processingSvcStub) ProcessArticle(_ context.Context, _ article.SourceArticle) (processing.ProcessedArticle, error) {
	return s.processed, nil
}

type processingStoreStub struct {
	record postgres.ProcessedArticleRecord
	err    error
}

func (s processingStoreStub) GetLatestByArticleID(_ context.Context, _ string) (postgres.ProcessedArticleRecord, error) {
	return s.record, s.err
}

func (s processingStoreStub) Save(_ context.Context, _ postgres.ProcessedArticleRecord) error {
	return nil
}

type dossierMaterializerStub struct {
	dossier dossier.ArticleDossier
}

func (s dossierMaterializerStub) Materialize(_ context.Context, _ service.MaterializeDossierInput) (dossier.ArticleDossier, error) {
	return s.dossier, nil
}

func TestRuntimeProcessingRunnerReturnsDossierDerivedCandidates(t *testing.T) {
	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articleFinderStub{article: article.SourceArticle{ID: "art-1", Title: "Model News"}},
		processingSvcStub{processed: processing.ProcessedArticle{Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"}, Analysis: processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8}}},
		processingStoreStub{err: gorm.ErrRecordNotFound},
		dossierMaterializerStub{dossier: dossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", TopicCategory: "AI", ImportanceScore: 0.8, RecommendationReason: "值得重点跟进", ReadingValue: "高", PriorityLevel: "high"}},
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
