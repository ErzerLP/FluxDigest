package service

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	ArticlePublishMode       string
	ArticleReviewMode        string
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

// Materialize 生成并持久化 dossier，同时初始化真实发布状态。
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
	materialized.Version = 0
	materialized.IsActive = true
	materialized.TranslationPromptVersion = input.TranslationPromptVersion
	materialized.AnalysisPromptVersion = input.AnalysisPromptVersion
	materialized.DossierPromptVersion = input.DossierPromptVersion
	materialized.LLMProfileVersion = input.LLMProfileVersion

	normalizedSuggestion, suggestionReasonCandidate := normalizePublishSuggestionValue(materialized.PublishSuggestion)
	if materialized.SuggestionReason == "" && suggestionReasonCandidate != "" {
		materialized.SuggestionReason = suggestionReasonCandidate
	}
	materialized.PublishSuggestion = normalizedSuggestion

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

	publishState := initialArticlePublishState(input.ArticlePublishMode, input.ArticleReviewMode, materialized.PublishSuggestion)
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
	out.ContentPolishedMarkdown = appendDossierSourceBlock(out.ContentPolishedMarkdown, source)
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

const (
	publishSuggestionSuggested = "suggested"
	publishSuggestionDraft     = "draft"
	defaultPublishState        = publishSuggestionDraft
)

var (
	positivePublishSuggestionPatterns = []*regexp.Regexp{
		mustCompilePublishSuggestionPattern(`(?i)\brecommend(?:ed)?\s+(?:to\s+)?publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\brecommend(?:ed)?\s+for\s+publish(?:ing|ation)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bshould\s+publish\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bshould\s+go\s+to\s+digest\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bworth\s+publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bworth\s+including\s+in\s+the?\s*digest\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bready\s+to\s+publish\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bpublish\s+now\b`),
		mustCompilePublishSuggestionPattern(`建议.{0,8}(发布|收录|纳入日报|进日报)`),
		mustCompilePublishSuggestionPattern(`推荐.{0,8}(发布|收录|纳入日报|进日报)`),
		mustCompilePublishSuggestionPattern(`值得.{0,8}(发布|收录|纳入日报|进日报)`),
		mustCompilePublishSuggestionPattern(`适合.{0,8}(发布|收录|纳入日报|进日报)`),
		mustCompilePublishSuggestionPattern(`(纳入日报|进日报)`),
	}
	negativePublishSuggestionPatterns = []*regexp.Regexp{
		mustCompilePublishSuggestionPattern(`(?i)\b(do\s+not|don't|dont|not)\s+publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bnot\s+ready\s+to\s+publish\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bnot\s+for\s+publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bwait\s+before\s+publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bhold\b.*\bpublish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bskip\s+publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bdraft\s+only\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bkeep\s+as\s+draft\b`),
		mustCompilePublishSuggestionPattern(`(?i)\bready\s+for\s+review\b.*\bnot\s+for\s+publish(?:ing)?\b`),
		mustCompilePublishSuggestionPattern(`不建议.{0,8}(发布|收录|纳入日报|进日报)?`),
		mustCompilePublishSuggestionPattern(`不推荐.{0,8}(发布|收录|纳入日报|进日报)?`),
		mustCompilePublishSuggestionPattern(`不值得.{0,8}(发布|收录|纳入日报|进日报)?`),
		mustCompilePublishSuggestionPattern(`暂不.{0,8}(发布|收录|纳入日报|进日报)?`),
		mustCompilePublishSuggestionPattern(`暂缓.{0,8}(发布|收录|纳入日报|进日报)?`),
		mustCompilePublishSuggestionPattern(`无需.{0,8}(发布|收录|纳入日报|进日报)?`),
		mustCompilePublishSuggestionPattern(`不纳入日报`),
		mustCompilePublishSuggestionPattern(`不进日报`),
		mustCompilePublishSuggestionPattern(`不发`),
	}
	allowedPublishStates = map[string]struct{}{
		publishSuggestionDraft:  {},
		publishStatePendingReview: {},
		publishStateQueued:        {},
		"publishing":              {},
		"published":               {},
		"failed":                  {},
	}
)

func normalizePublishSuggestionValue(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return publishSuggestionDraft, ""
	}

	lower := strings.ToLower(trimmed)
	normalized := publishSuggestionDraft
	switch lower {
	case publishSuggestionSuggested:
		normalized = publishSuggestionSuggested
	case publishSuggestionDraft, "hold", "ignore":
		normalized = publishSuggestionDraft
	default:
		if boolValue, err := strconv.ParseBool(lower); err == nil {
			if boolValue {
				normalized = publishSuggestionSuggested
			}
		} else if matchesPublishSuggestionPattern(trimmed, negativePublishSuggestionPatterns) {
			normalized = publishSuggestionDraft
		} else if matchesPublishSuggestionPattern(trimmed, positivePublishSuggestionPatterns) {
			normalized = publishSuggestionSuggested
		}
	}

	reason := ""
	if looksLikeNaturalLanguageSuggestion(trimmed) {
		reason = trimmed
	}

	return normalized, reason
}

func normalizePublishStateValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultPublishState
	}
	lower := strings.ToLower(trimmed)
	if _, ok := allowedPublishStates[lower]; ok {
		return lower
	}
	return defaultPublishState
}

func looksLikeNaturalLanguageSuggestion(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "\n") {
		return true
	}
	if utf8.RuneCountInString(trimmed) >= 32 {
		return true
	}
	if strings.Count(trimmed, " ") >= 2 {
		return true
	}
	if strings.ContainsAny(trimmed, "。！？!?") {
		return true
	}
	if strings.ContainsAny(trimmed, "，；、：") && utf8.RuneCountInString(trimmed) >= 12 {
		return true
	}
	return false
}

func matchesPublishSuggestionPattern(text string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func appendDossierSourceBlock(content string, source article.SourceArticle) string {
	trimmed := strings.TrimSpace(content)
	if trimmed != "" && strings.Contains(trimmed, "## 原文来源") && (source.URL == "" || strings.Contains(trimmed, source.URL)) {
		return trimmed
	}

	lines := make([]string, 0, 4)
	if feedTitle := strings.TrimSpace(source.FeedTitle); feedTitle != "" {
		lines = append(lines, "- 订阅源："+feedTitle)
	}
	if author := strings.TrimSpace(source.Author); author != "" {
		lines = append(lines, "- 作者："+author)
	}
	if url := strings.TrimSpace(source.URL); url != "" {
		lines = append(lines, "- 原文链接："+url)
	}
	if len(lines) == 0 {
		return trimmed
	}

	sourceBlock := "## 原文来源\n" + strings.Join(lines, "\n")
	if trimmed == "" {
		return sourceBlock
	}
	return trimmed + "\n\n---\n\n" + sourceBlock
}

func mustCompilePublishSuggestionPattern(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
