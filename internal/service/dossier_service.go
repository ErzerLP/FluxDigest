package service

import (
	"context"
	"strings"
	"time"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/dossier"
	"rss-platform/internal/repository/postgres"
)

const dossierDateLayout = "2006-01-02"

// BuildDossierInput 表示构建 dossier 的上下文。
type BuildDossierInput struct {
	Article    article.SourceArticle
	Processing postgres.ProcessedArticleRecord
}

// MaterializeDossierInput 表示物化 dossier 所需输入。
type MaterializeDossierInput struct {
	Article                  article.SourceArticle
	Processing               postgres.ProcessedArticleRecord
	ArticleID                string
	ProcessingID             string
	DigestDate               string
	TitleTranslated          string
	SummaryTranslated        string
	CoreSummary              string
	KeyPoints                []string
	TopicCategory            string
	ImportanceScore          float64
	ContentTranslated        string
	TranslationPromptVersion int
	AnalysisPromptVersion    int
	DossierPromptVersion     int
	LLMProfileVersion        int
}

// DossierBuilder 定义 dossier 构建能力。
type DossierBuilder interface {
	Build(ctx context.Context, input BuildDossierInput) (dossier.ArticleDossier, error)
}

// DossierRepository 定义 dossier 持久化能力。
type DossierRepository interface {
	SaveActive(ctx context.Context, input postgres.ArticleDossierRecord) (postgres.ArticleDossierRecord, error)
}

// PublishStateRepository 定义发布状态写入能力。
type PublishStateRepository interface {
	Upsert(ctx context.Context, input postgres.ArticlePublishStateRecord) error
}

// DossierService 负责编排 dossier 物化与发布建议状态。
type DossierService struct {
	builder      DossierBuilder
	dossiers     DossierRepository
	publishState PublishStateRepository
}

// NewDossierService 创建 DossierService。
func NewDossierService(builder DossierBuilder, dossiers DossierRepository, publishState PublishStateRepository) *DossierService {
	return &DossierService{builder: builder, dossiers: dossiers, publishState: publishState}
}

// Materialize 生成并持久化 dossier，同时写入建议发布状态。
func (s *DossierService) Materialize(ctx context.Context, input MaterializeDossierInput) (dossier.ArticleDossier, error) {
	articleInput := input.Article
	if articleInput.ID == "" {
		articleInput.ID = input.ArticleID
	}

	processingInput := input.Processing
	if processingInput.ArticleID == "" {
		processingInput.ArticleID = input.ArticleID
	}
	if processingInput.ID == "" {
		processingInput.ID = input.ProcessingID
	}
	if processingInput.TitleTranslated == "" {
		processingInput.TitleTranslated = input.TitleTranslated
	}
	if processingInput.SummaryTranslated == "" {
		processingInput.SummaryTranslated = input.SummaryTranslated
	}
	if processingInput.ContentTranslated == "" {
		processingInput.ContentTranslated = input.ContentTranslated
	}
	if processingInput.CoreSummary == "" {
		processingInput.CoreSummary = input.CoreSummary
	}
	if len(processingInput.KeyPoints) == 0 {
		processingInput.KeyPoints = append([]string(nil), input.KeyPoints...)
	}
	if processingInput.TopicCategory == "" {
		processingInput.TopicCategory = input.TopicCategory
	}
	if processingInput.ImportanceScore == 0 {
		processingInput.ImportanceScore = input.ImportanceScore
	}
	if processingInput.TranslationPromptVersion == 0 {
		processingInput.TranslationPromptVersion = input.TranslationPromptVersion
	}
	if processingInput.AnalysisPromptVersion == 0 {
		processingInput.AnalysisPromptVersion = input.AnalysisPromptVersion
	}
	if processingInput.LLMProfileVersion == 0 {
		processingInput.LLMProfileVersion = input.LLMProfileVersion
	}

	built, err := s.builder.Build(ctx, BuildDossierInput{Article: articleInput, Processing: processingInput})
	if err != nil {
		return dossier.ArticleDossier{}, err
	}

	digestDate, err := time.ParseInLocation(dossierDateLayout, input.DigestDate, shanghaiLocation())
	if err != nil {
		return dossier.ArticleDossier{}, err
	}

	materialized := fillDossierDefaults(built, articleInput, processingInput)
	materialized.ArticleID = input.ArticleID
	materialized.ProcessingID = input.ProcessingID
	materialized.DigestDate = digestDate
	materialized.Version = 1
	materialized.IsActive = true
	materialized.TranslationPromptVersion = input.TranslationPromptVersion
	materialized.AnalysisPromptVersion = input.AnalysisPromptVersion
	materialized.DossierPromptVersion = input.DossierPromptVersion
	materialized.LLMProfileVersion = input.LLMProfileVersion

	saved, err := s.dossiers.SaveActive(ctx, postgres.ArticleDossierRecord{
		ID:                       materialized.ID,
		ArticleID:                materialized.ArticleID,
		ProcessingID:             materialized.ProcessingID,
		DigestDate:               materialized.DigestDate.Format(dossierDateLayout),
		Version:                  materialized.Version,
		IsActive:                 materialized.IsActive,
		TitleTranslated:          materialized.TitleTranslated,
		SummaryPolished:          materialized.SummaryPolished,
		CoreSummary:              materialized.CoreSummary,
		KeyPoints:                materialized.KeyPoints,
		TopicCategory:            materialized.TopicCategory,
		ImportanceScore:          materialized.ImportanceScore,
		RecommendationReason:     materialized.RecommendationReason,
		ReadingValue:             materialized.ReadingValue,
		PriorityLevel:            materialized.PriorityLevel,
		ContentPolishedMarkdown:  materialized.ContentPolishedMarkdown,
		AnalysisLongformMarkdown: materialized.AnalysisLongformMarkdown,
		BackgroundContext:        materialized.BackgroundContext,
		ImpactAnalysis:           materialized.ImpactAnalysis,
		DebatePoints:             materialized.DebatePoints,
		TargetAudience:           materialized.TargetAudience,
		PublishSuggestion:        materialized.PublishSuggestion,
		SuggestionReason:         materialized.SuggestionReason,
		SuggestedChannels:        materialized.SuggestedChannels,
		SuggestedTags:            materialized.SuggestedTags,
		SuggestedCategories:      materialized.SuggestedCategories,
		TranslationPromptVersion: materialized.TranslationPromptVersion,
		AnalysisPromptVersion:    materialized.AnalysisPromptVersion,
		DossierPromptVersion:     materialized.DossierPromptVersion,
		LLMProfileVersion:        materialized.LLMProfileVersion,
		CreatedAt:                materialized.CreatedAt,
		UpdatedAt:                materialized.UpdatedAt,
	})
	if err != nil {
		return dossier.ArticleDossier{}, err
	}

	materialized.ID = saved.ID
	materialized.IsActive = saved.IsActive
	materialized.Version = saved.Version
	materialized.CreatedAt = saved.CreatedAt
	materialized.UpdatedAt = saved.UpdatedAt

	publishState := materialized.PublishSuggestion
	if strings.TrimSpace(publishState) == "" {
		publishState = "draft"
	}
	if err := s.publishState.Upsert(ctx, postgres.ArticlePublishStateRecord{
		DossierID: materialized.ID,
		State:     publishState,
	}); err != nil {
		return dossier.ArticleDossier{}, err
	}

	return materialized, nil
}

func fillDossierDefaults(built dossier.ArticleDossier, source article.SourceArticle, processed postgres.ProcessedArticleRecord) dossier.ArticleDossier {
	out := built
	if strings.TrimSpace(out.TitleTranslated) == "" {
		out.TitleTranslated = firstNonEmpty(processed.TitleTranslated, source.Title)
	}
	if strings.TrimSpace(out.SummaryPolished) == "" {
		out.SummaryPolished = processed.SummaryTranslated
	}
	if strings.TrimSpace(out.CoreSummary) == "" {
		out.CoreSummary = processed.CoreSummary
	}
	if len(out.KeyPoints) == 0 {
		out.KeyPoints = copyStringSlice(processed.KeyPoints)
	}
	if strings.TrimSpace(out.TopicCategory) == "" {
		out.TopicCategory = processed.TopicCategory
	}
	if out.ImportanceScore == 0 {
		out.ImportanceScore = processed.ImportanceScore
	}
	if strings.TrimSpace(out.ContentPolishedMarkdown) == "" {
		out.ContentPolishedMarkdown = processed.ContentTranslated
	}
	if strings.TrimSpace(out.AnalysisLongformMarkdown) == "" {
		out.AnalysisLongformMarkdown = out.CoreSummary
	}
	if strings.TrimSpace(out.PriorityLevel) == "" {
		out.PriorityLevel = "normal"
	}
	if strings.TrimSpace(out.PublishSuggestion) == "" {
		out.PublishSuggestion = "draft"
	}
	out.SuggestedChannels = copyStringSlice(out.SuggestedChannels)
	out.SuggestedTags = copyStringSlice(out.SuggestedTags)
	out.SuggestedCategories = copyStringSlice(out.SuggestedCategories)
	out.DebatePoints = copyStringSlice(out.DebatePoints)
	out.KeyPoints = copyStringSlice(out.KeyPoints)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func copyStringSlice(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
