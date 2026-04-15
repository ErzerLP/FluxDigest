package service_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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
	marked  [][]int64
}

func (s entryListerStub) ListEntries(_ context.Context, _, _ time.Time) ([]miniflux.Entry, error) {
	return s.entries, nil
}

func (s *entryListerStub) MarkEntriesRead(_ context.Context, entryIDs []int64) error {
	copied := append([]int64(nil), entryIDs...)
	s.marked = append(s.marked, copied)
	return nil
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

type blockingProcessingStub struct {
	start     chan struct{}
	release   chan struct{}
	processed processing.ProcessedArticle
}

func (s *blockingProcessingStub) ProcessArticle(ctx context.Context, _ article.SourceArticle) (processing.ProcessedArticle, error) {
	select {
	case s.start <- struct{}{}:
	case <-ctx.Done():
		return processing.ProcessedArticle{}, ctx.Err()
	}
	select {
	case <-s.release:
	case <-ctx.Done():
		return processing.ProcessedArticle{}, ctx.Err()
	}
	return s.processed, nil
}

type mapArticleFinder struct {
	byEntry map[int64]article.SourceArticle
	byID    map[string]article.SourceArticle
}

func (s *mapArticleFinder) FindByMinifluxEntryID(_ context.Context, entryID int64) (article.SourceArticle, error) {
	if article, ok := s.byEntry[entryID]; ok {
		return article, nil
	}
	return article.SourceArticle{}, errors.New("article not found")
}

func (s *mapArticleFinder) FindByID(_ context.Context, id string) (article.SourceArticle, error) {
	if article, ok := s.byID[id]; ok {
		return article, nil
	}
	for _, article := range s.byEntry {
		if article.ID == id {
			return article, nil
		}
	}
	return article.SourceArticle{}, errors.New("article not found")
}

type errorProcessingStub struct {
	failOn    string
	fail      error
	blocked   chan struct{}
	processed processing.ProcessedArticle
	calls     int32
}

func (s *errorProcessingStub) ProcessArticle(ctx context.Context, input article.SourceArticle) (processing.ProcessedArticle, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.fail != nil && input.ID == s.failOn {
		return processing.ProcessedArticle{}, s.fail
	}
	select {
	case <-ctx.Done():
		return processing.ProcessedArticle{}, ctx.Err()
	case <-s.blocked:
		return s.processed, nil
	}
}

func TestRuntimeProcessingRunnerReturnsDossierDerivedCandidates(t *testing.T) {
	lister := &entryListerStub{entries: []miniflux.Entry{{ID: 101}}}
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", TopicCategory: "AI", ImportanceScore: 0.8, RecommendationReason: "值得重点跟进", ReadingValue: "高", PriorityLevel: "high"}}
	finder := &articleFinderStub{article: article.SourceArticle{ID: "art-1", Title: "Model News"}}
	processor := &processingSvcStub{processed: processing.ProcessedArticle{Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"}, Analysis: processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8}}}
	runner := service.NewRuntimeProcessingRunner(
		lister,
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
	if len(lister.marked) != 1 || len(lister.marked[0]) != 1 || lister.marked[0][0] != 101 {
		t.Fatalf("expected processed entry marked read, got %#v", lister.marked)
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

func TestRuntimeProcessingRunnerProcessesEntriesConcurrently(t *testing.T) {
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1"}}
	finder := &articleFinderStub{article: article.SourceArticle{ID: "art-1", Title: "Model News"}}
	startCh := make(chan struct{}, 2)
	releaseCh := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseCh) })
	}
	defer release()
	processor := &blockingProcessingStub{
		start:     startCh,
		release:   releaseCh,
		processed: processing.ProcessedArticle{Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"}, Analysis: processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8}},
	}
	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 201}, {ID: 202}}},
		finder,
		processor,
		store,
		materializer,
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)
	runner.SetConcurrencyCalculator(func(int) int { return 2 })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runner.ProcessPending(ctx, time.Now().Add(-time.Hour), time.Now())
		done <- err
	}()

	waitForStart := func() {
		select {
		case <-startCh:
		case <-time.After(time.Second):
			release()
			t.Fatalf("expected concurrent processing start")
		}
	}

	waitForStart()
	waitForStart()
	release()

	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRuntimeProcessingRunnerPreservesEntryOrderWithDuplicates(t *testing.T) {
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{}}
	finder := &mapArticleFinder{
		byEntry: map[int64]article.SourceArticle{
			301: {ID: "art-301", Title: "A"},
			302: {ID: "art-302", Title: "B"},
		},
		byID: map[string]article.SourceArticle{
			"art-301": {ID: "art-301", Title: "A"},
			"art-302": {ID: "art-302", Title: "B"},
		},
	}
	processor := &processingSvcStub{processed: processing.ProcessedArticle{Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"}, Analysis: processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8}}}
	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 301}, {ID: 302}, {ID: 301}}},
		finder,
		processor,
		store,
		materializer,
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)
	runner.SetConcurrencyCalculator(func(int) int { return 2 })

	candidates, err := runner.ProcessPending(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 2 {
		t.Fatalf("expected two candidates, got %d", len(candidates))
	}
	if candidates[0].ID != "art-301" || candidates[1].ID != "art-302" {
		t.Fatalf("unexpected order %v", candidates)
	}
}

var processingError = errors.New("processing failed")

func TestRuntimeProcessingRunnerShortCircuitsOnProcessingError(t *testing.T) {
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}
	materializer := &dossierMaterializerStub{dossier: dossier.ArticleDossier{}}
	finder := &mapArticleFinder{
		byEntry: map[int64]article.SourceArticle{
			401: {ID: "art-401", Title: "A"},
			402: {ID: "art-402", Title: "B"},
		},
		byID: map[string]article.SourceArticle{
			"art-401": {ID: "art-401", Title: "A"},
			"art-402": {ID: "art-402", Title: "B"},
		},
	}
	processor := &errorProcessingStub{
		failOn:  "art-401",
		fail:    processingError,
		blocked: make(chan struct{}),
		processed: processing.ProcessedArticle{
			Translation: processing.Translation{TitleTranslated: "模型新闻", SummaryTranslated: "摘要", ContentTranslated: "正文"},
			Analysis:    processing.Analysis{CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.8},
		},
	}
	defer func() {
		select {
		case <-processor.blocked:
		default:
			close(processor.blocked)
		}
	}()

	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 401}, {ID: 402}}},
		finder,
		processor,
		store,
		materializer,
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)
	runner.SetConcurrencyCalculator(func(int) int { return 2 })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := runner.ProcessPending(ctx, time.Now().Add(-time.Hour), time.Now())
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, processingError) {
			t.Fatalf("expected processing error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		close(processor.blocked)
		t.Fatal("expected processing runner to return on error without waiting for blocked tasks")
	}
}
