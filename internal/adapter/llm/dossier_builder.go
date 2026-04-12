package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"text/template"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/dossier"
	"rss-platform/internal/repository/postgres"
)

// DossierBuildInput 表示 dossier 生成输入。
type DossierBuildInput struct {
	Article    article.SourceArticle
	Processing postgres.ProcessedArticleRecord
}

// DossierBuilder 负责调用 LLM 生成 dossier 字段。
type DossierBuilder struct {
	chat     ChatInvoker
	template promptTemplate
}

// NewDossierBuilder 创建 dossier 生成器。
func NewDossierBuilder(chat ChatInvoker, templatePath string) *DossierBuilder {
	return &DossierBuilder{
		chat:     chat,
		template: promptTemplate{path: templatePath},
	}
}

// NewDossierBuilderFromTemplateText 创建内联模板的 dossier 生成器。
func NewDossierBuilderFromTemplateText(chat ChatInvoker, templateText string) *DossierBuilder {
	return &DossierBuilder{
		chat:     chat,
		template: promptTemplate{text: templateText},
	}
}

// Build 生成文章 dossier。
func (b *DossierBuilder) Build(ctx context.Context, input DossierBuildInput) (dossier.ArticleDossier, error) {
	if b == nil || b.chat == nil {
		return dossier.ArticleDossier{}, errChatInvokerRequired
	}

	prompt, err := b.buildPrompt(input)
	if err != nil {
		return dossier.ArticleDossier{}, err
	}

	raw, err := b.chat.Generate(ctx, prompt)
	if err != nil {
		return dossier.ArticleDossier{}, err
	}

	var out struct {
		TitleTranslated          string           `json:"title_translated"`
		SummaryPolished          string           `json:"summary_polished"`
		CoreSummary              string           `json:"core_summary"`
		KeyPoints                []string         `json:"key_points"`
		TopicCategory            string           `json:"topic_category"`
		ImportanceScore          float64          `json:"importance_score"`
		RecommendationReason     string           `json:"recommendation_reason"`
		ReadingValue             string           `json:"reading_value"`
		PriorityLevel            string           `json:"priority_level"`
		ContentPolishedMarkdown  string           `json:"content_polished_markdown"`
		AnalysisLongformMarkdown string           `json:"analysis_longform_markdown"`
		BackgroundContext        string           `json:"background_context"`
		ImpactAnalysis           string           `json:"impact_analysis"`
		DebatePoints             []string         `json:"debate_points"`
		TargetAudience           normalizedString `json:"target_audience"`
		PublishSuggestion        string           `json:"publish_suggestion"`
		SuggestionReason         string           `json:"suggestion_reason"`
		SuggestedChannels        []string         `json:"suggested_channels"`
		SuggestedTags            []string         `json:"suggested_tags"`
		SuggestedCategories      []string         `json:"suggested_categories"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONObject(raw)), &out); err != nil {
		return dossier.ArticleDossier{}, err
	}

	return dossier.ArticleDossier{
		TitleTranslated:          out.TitleTranslated,
		SummaryPolished:          out.SummaryPolished,
		CoreSummary:              out.CoreSummary,
		KeyPoints:                out.KeyPoints,
		TopicCategory:            out.TopicCategory,
		ImportanceScore:          normalizeImportanceScore(out.ImportanceScore),
		RecommendationReason:     out.RecommendationReason,
		ReadingValue:             out.ReadingValue,
		PriorityLevel:            out.PriorityLevel,
		ContentPolishedMarkdown:  out.ContentPolishedMarkdown,
		AnalysisLongformMarkdown: out.AnalysisLongformMarkdown,
		BackgroundContext:        out.BackgroundContext,
		ImpactAnalysis:           out.ImpactAnalysis,
		DebatePoints:             out.DebatePoints,
		TargetAudience:           string(out.TargetAudience),
		PublishSuggestion:        out.PublishSuggestion,
		SuggestionReason:         out.SuggestionReason,
		SuggestedChannels:        out.SuggestedChannels,
		SuggestedTags:            out.SuggestedTags,
		SuggestedCategories:      out.SuggestedCategories,
	}, nil
}

type normalizedString string

func (s *normalizedString) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = normalizedString(strings.TrimSpace(single))
		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		filtered := make([]string, 0, len(list))
		for _, item := range list {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			filtered = append(filtered, trimmed)
		}
		*s = normalizedString(strings.Join(filtered, ", "))
		return nil
	}

	return errors.New("expected string or string array")
}

func (b *DossierBuilder) buildPrompt(input DossierBuildInput) (string, error) {
	templateText, err := loadTemplate(b.template)
	if err != nil {
		return "", err
	}

	name := filepath.Base(b.template.path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "inline-template"
	}

	tmpl, err := template.New(name).Parse(templateText)
	if err != nil {
		return "", err
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, input); err != nil {
		return "", err
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	var prompt strings.Builder
	prompt.WriteString(strings.TrimSpace(rendered.String()))
	prompt.WriteString("\n")
	prompt.WriteString("仅输出 JSON。输入：")
	prompt.Write(payload)
	return prompt.String(), nil
}

var errDossierBuilderRequired = errors.New("dossier builder is required")
