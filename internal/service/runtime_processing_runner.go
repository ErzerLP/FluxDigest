package service

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"rss-platform/internal/adapter/miniflux"
	"rss-platform/internal/domain/article"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/domain/dossier"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/repository/postgres"
)

// RuntimePromptVersions 表示运行时 prompt/profile 版本号。
type RuntimePromptVersions struct {
	Translation int
	Analysis    int
	Dossier     int
	LLM         int
}

// RuntimeProcessingRunner 负责文章处理与 dossier 物化。
type RuntimeProcessingRunner struct {
	client                runtimeEntryLister
	articles              runtimeArticleFinder
	processing            runtimeArticleProcessor
	results               runtimeProcessingStore
	dossiers              runtimeDossierMaterializer
	versions              RuntimePromptVersions
	concurrencyCalculator func(int) int
}

// NewRuntimeProcessingRunner 创建 RuntimeProcessingRunner。
func NewRuntimeProcessingRunner(
	client runtimeEntryLister,
	articles runtimeArticleFinder,
	processing runtimeArticleProcessor,
	results runtimeProcessingStore,
	dossiers runtimeDossierMaterializer,
	versions RuntimePromptVersions,
) *RuntimeProcessingRunner {
	return &RuntimeProcessingRunner{
		client:                client,
		articles:              articles,
		processing:            processing,
		results:               results,
		dossiers:              dossiers,
		versions:              versions,
		concurrencyCalculator: defaultRuntimeConcurrency,
	}
}

// ProcessPending 处理窗口内文章并返回 dossier 派生候选。
func (r *RuntimeProcessingRunner) ProcessPending(ctx context.Context, windowStart, windowEnd time.Time) ([]domaindigest.CandidateArticle, error) {
	entries, err := r.client.ListEntries(ctx, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	seen := make(map[int64]struct{}, len(entries))
	jobs := make([]runtimeProcessingJob, 0, len(entries))
	for _, entry := range entries {
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		seen[entry.ID] = struct{}{}
		jobs = append(jobs, runtimeProcessingJob{order: len(jobs), entryID: entry.ID})
	}

	if len(jobs) == 0 {
		return nil, nil
	}

	workerCount := r.concurrencyCalculator(len(jobs))
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(jobs) {
		workerCount = len(jobs)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan runtimeProcessingJob)
	resultCh := make(chan runtimeProcessingResult, len(jobs))
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if err := ctx.Err(); err != nil {
					return
				}

				candidate, jobErr := r.runProcessingJob(ctx, job, windowStart)
				if jobErr != nil {
					select {
					case resultCh <- runtimeProcessingResult{err: jobErr}:
					case <-ctx.Done():
					}
					cancel()
					return
				}

				select {
				case resultCh <- runtimeProcessingResult{order: job.order, candidate: candidate}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for _, job := range jobs {
			select {
			case <-ctx.Done():
				return
			case jobCh <- job:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	candidates := make([]domaindigest.CandidateArticle, len(jobs))
	received := 0
	var firstErr error
	for res := range resultCh {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		candidates[res.order] = res.candidate
		received++
	}

	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if received != len(jobs) {
		return nil, errors.New("runtimeProcessingRunner: incomplete processing results")
	}

	return candidates, nil
}

// ReprocessArticle 重新处理指定文章，并重建对应 dossier。
func (r *RuntimeProcessingRunner) ReprocessArticle(ctx context.Context, articleID string, force bool) error {
	_ = force

	source, err := r.articles.FindByID(ctx, articleID)
	if err != nil {
		return err
	}

	processed, err := r.processing.ProcessArticle(ctx, source)
	if err != nil {
		return err
	}

	record := postgres.ProcessedArticleRecord{
		ID:                       newProcessingID(),
		ArticleID:                source.ID,
		TitleTranslated:          processed.Translation.TitleTranslated,
		SummaryTranslated:        processed.Translation.SummaryTranslated,
		ContentTranslated:        processed.Translation.ContentTranslated,
		CoreSummary:              processed.Analysis.CoreSummary,
		KeyPoints:                processed.Analysis.KeyPoints,
		TopicCategory:            processed.Analysis.TopicCategory,
		ImportanceScore:          processed.Analysis.ImportanceScore,
		TranslationPromptVersion: r.versions.Translation,
		AnalysisPromptVersion:    r.versions.Analysis,
		LLMProfileVersion:        r.versions.LLM,
	}
	if err := r.results.Save(ctx, record); err != nil {
		return err
	}

	_, err = r.dossiers.Materialize(ctx, MaterializeDossierInput{
		Article:                  source,
		Processing:               record,
		ArticleID:                source.ID,
		ProcessingID:             record.ID,
		DigestDate:               time.Now().In(shanghaiLocation()).Format(dossierDateLayout),
		TitleTranslated:          record.TitleTranslated,
		SummaryTranslated:        record.SummaryTranslated,
		CoreSummary:              record.CoreSummary,
		KeyPoints:                record.KeyPoints,
		TopicCategory:            record.TopicCategory,
		ImportanceScore:          record.ImportanceScore,
		ContentTranslated:        record.ContentTranslated,
		TranslationPromptVersion: r.versions.Translation,
		AnalysisPromptVersion:    r.versions.Analysis,
		DossierPromptVersion:     r.versions.Dossier,
		LLMProfileVersion:        r.versions.LLM,
	})
	return err
}

type runtimeProcessingJob struct {
	order   int
	entryID int64
}

type runtimeProcessingResult struct {
	order     int
	candidate domaindigest.CandidateArticle
	err       error
}

func (r *RuntimeProcessingRunner) runProcessingJob(ctx context.Context, job runtimeProcessingJob, windowStart time.Time) (domaindigest.CandidateArticle, error) {
	if err := ctx.Err(); err != nil {
		return domaindigest.CandidateArticle{}, err
	}

	source, err := r.articles.FindByMinifluxEntryID(ctx, job.entryID)
	if err != nil {
		return domaindigest.CandidateArticle{}, err
	}

	record, recordErr := r.results.GetLatestByArticleID(ctx, source.ID)
	if recordErr != nil && !errors.Is(recordErr, gorm.ErrRecordNotFound) {
		return domaindigest.CandidateArticle{}, recordErr
	}

	if errors.Is(recordErr, gorm.ErrRecordNotFound) {
		processed, processErr := r.processing.ProcessArticle(ctx, source)
		if processErr != nil {
			return domaindigest.CandidateArticle{}, processErr
		}

		record = postgres.ProcessedArticleRecord{
			ID:                       newProcessingID(),
			ArticleID:                source.ID,
			TitleTranslated:          processed.Translation.TitleTranslated,
			SummaryTranslated:        processed.Translation.SummaryTranslated,
			ContentTranslated:        processed.Translation.ContentTranslated,
			CoreSummary:              processed.Analysis.CoreSummary,
			KeyPoints:                processed.Analysis.KeyPoints,
			TopicCategory:            processed.Analysis.TopicCategory,
			ImportanceScore:          processed.Analysis.ImportanceScore,
			TranslationPromptVersion: r.versions.Translation,
			AnalysisPromptVersion:    r.versions.Analysis,
			LLMProfileVersion:        r.versions.LLM,
		}

		if err := ctx.Err(); err != nil {
			return domaindigest.CandidateArticle{}, err
		}
		if saveErr := r.results.Save(ctx, record); saveErr != nil {
			return domaindigest.CandidateArticle{}, saveErr
		}
	}

	if err := ctx.Err(); err != nil {
		return domaindigest.CandidateArticle{}, err
	}

	dossierItem, matErr := r.dossiers.Materialize(ctx, MaterializeDossierInput{
		Article:                  source,
		Processing:               record,
		ArticleID:                source.ID,
		ProcessingID:             record.ID,
		DigestDate:               windowStart.In(shanghaiLocation()).Format(dossierDateLayout),
		TitleTranslated:          record.TitleTranslated,
		SummaryTranslated:        record.SummaryTranslated,
		CoreSummary:              record.CoreSummary,
		KeyPoints:                record.KeyPoints,
		TopicCategory:            record.TopicCategory,
		ImportanceScore:          record.ImportanceScore,
		ContentTranslated:        record.ContentTranslated,
		TranslationPromptVersion: r.versions.Translation,
		AnalysisPromptVersion:    r.versions.Analysis,
		DossierPromptVersion:     r.versions.Dossier,
		LLMProfileVersion:        r.versions.LLM,
	})
	if matErr != nil {
		return domaindigest.CandidateArticle{}, matErr
	}

	if err := ctx.Err(); err != nil {
		return domaindigest.CandidateArticle{}, err
	}

	return candidateFromDossier(source, record, dossierItem), nil
}

func (r *RuntimeProcessingRunner) SetConcurrencyCalculator(fn func(int) int) {
	if r == nil {
		return
	}
	if fn == nil {
		r.concurrencyCalculator = defaultRuntimeConcurrency
		return
	}
	r.concurrencyCalculator = fn
}

func defaultRuntimeConcurrency(jobCount int) int {
	if jobCount <= 0 {
		return 1
	}
	cpuCount := runtime.NumCPU()
	if cpuCount <= 0 {
		cpuCount = 1
	}
	if jobCount < cpuCount {
		return jobCount
	}
	return cpuCount
}

type runtimeEntryLister interface {
	ListEntries(ctx context.Context, windowStart, windowEnd time.Time) ([]miniflux.Entry, error)
}

type runtimeArticleFinder interface {
	FindByMinifluxEntryID(ctx context.Context, minifluxEntryID int64) (article.SourceArticle, error)
	FindByID(ctx context.Context, articleID string) (article.SourceArticle, error)
}

type runtimeArticleProcessor interface {
	ProcessArticle(ctx context.Context, input article.SourceArticle) (processing.ProcessedArticle, error)
}

type runtimeProcessingStore interface {
	GetLatestByArticleID(ctx context.Context, articleID string) (postgres.ProcessedArticleRecord, error)
	Save(ctx context.Context, input postgres.ProcessedArticleRecord) error
}

type runtimeDossierMaterializer interface {
	Materialize(ctx context.Context, input MaterializeDossierInput) (dossier.ArticleDossier, error)
}

func candidateFromDossier(source article.SourceArticle, processed postgres.ProcessedArticleRecord, item dossier.ArticleDossier) domaindigest.CandidateArticle {
	title := firstNonEmpty(item.TitleTranslated, processed.TitleTranslated, source.Title)
	coreSummary := firstNonEmpty(item.CoreSummary, processed.CoreSummary)
	summaryPolished := firstNonEmpty(item.SummaryPolished, processed.SummaryTranslated)
	topicCategory := firstNonEmpty(item.TopicCategory, processed.TopicCategory)

	importanceScore := item.ImportanceScore
	if importanceScore == 0 {
		importanceScore = processed.ImportanceScore
	}

	return domaindigest.CandidateArticle{
		ID:                   source.ID,
		DossierID:            item.ID,
		Title:                title,
		CoreSummary:          coreSummary,
		SummaryPolished:      summaryPolished,
		TopicCategory:        topicCategory,
		ImportanceScore:      importanceScore,
		RecommendationReason: item.RecommendationReason,
		ReadingValue:         item.ReadingValue,
		PriorityLevel:        item.PriorityLevel,
	}
}

func newProcessingID() string {
	orderedID, err := uuid.NewV7()
	if err == nil {
		return orderedID.String()
	}
	return uuid.NewString()
}
