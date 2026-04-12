package service

import (
	"context"
	"errors"
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
	client     runtimeEntryLister
	articles   runtimeArticleFinder
	processing runtimeArticleProcessor
	results    runtimeProcessingStore
	dossiers   runtimeDossierMaterializer
	versions   RuntimePromptVersions
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
		client:     client,
		articles:   articles,
		processing: processing,
		results:    results,
		dossiers:   dossiers,
		versions:   versions,
	}
}

// ProcessPending 处理窗口内文章并返回 dossier 派生候选。
func (r *RuntimeProcessingRunner) ProcessPending(ctx context.Context, windowStart, windowEnd time.Time) ([]domaindigest.CandidateArticle, error) {
	entries, err := r.client.ListEntries(ctx, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	candidates := make([]domaindigest.CandidateArticle, 0, len(entries))
	seen := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		seen[entry.ID] = struct{}{}

		source, err := r.articles.FindByMinifluxEntryID(ctx, entry.ID)
		if err != nil {
			return nil, err
		}

		record, err := r.results.GetLatestByArticleID(ctx, source.ID)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			processed, processErr := r.processing.ProcessArticle(ctx, source)
			if processErr != nil {
				return nil, processErr
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
			if saveErr := r.results.Save(ctx, record); saveErr != nil {
				return nil, saveErr
			}
		}

		dossierItem, err := r.dossiers.Materialize(ctx, MaterializeDossierInput{
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
		if err != nil {
			return nil, err
		}

		candidates = append(candidates, candidateFromDossier(source, record, dossierItem))
	}

	return candidates, nil
}

type runtimeEntryLister interface {
	ListEntries(ctx context.Context, windowStart, windowEnd time.Time) ([]miniflux.Entry, error)
}

type runtimeArticleFinder interface {
	FindByMinifluxEntryID(ctx context.Context, minifluxEntryID int64) (article.SourceArticle, error)
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
