package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
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

type structuredJSONDossierChatInvoker interface {
	GenerateStructuredJSON(ctx context.Context, prompt string) (string, error)
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

	raw, err := generateDossierStructuredJSON(ctx, b.chat, prompt)
	if err != nil {
		return dossier.ArticleDossier{}, err
	}

	var out struct {
		TitleTranslated          normalizedString            `json:"title_translated"`
		SummaryPolished          normalizedString            `json:"summary_polished"`
		CoreSummary              normalizedString            `json:"core_summary"`
		KeyPoints                normalizedStringList        `json:"key_points"`
		TopicCategory            normalizedString            `json:"topic_category"`
		ImportanceScore          float64                     `json:"importance_score"`
		RecommendationReason     normalizedString            `json:"recommendation_reason"`
		ReadingValue             normalizedString            `json:"reading_value"`
		PriorityLevel            normalizedString            `json:"priority_level"`
		ContentPolishedMarkdown  normalizedString            `json:"content_polished_markdown"`
		AnalysisLongformMarkdown normalizedString            `json:"analysis_longform_markdown"`
		BackgroundContext        normalizedString            `json:"background_context"`
		ImpactAnalysis           normalizedString            `json:"impact_analysis"`
		DebatePoints             normalizedStringList        `json:"debate_points"`
		TargetAudience           normalizedString            `json:"target_audience"`
		PublishSuggestion        normalizedPublishSuggestion `json:"publish_suggestion"`
		SuggestionReason         normalizedString            `json:"suggestion_reason"`
		SuggestedChannels        normalizedStringList        `json:"suggested_channels"`
		SuggestedTags            normalizedStringList        `json:"suggested_tags"`
		SuggestedCategories      normalizedStringList        `json:"suggested_categories"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONObject(raw)), &out); err != nil {
		return dossier.ArticleDossier{}, err
	}

	return dossier.ArticleDossier{
		TitleTranslated:          string(out.TitleTranslated),
		SummaryPolished:          string(out.SummaryPolished),
		CoreSummary:              string(out.CoreSummary),
		KeyPoints:                []string(out.KeyPoints),
		TopicCategory:            string(out.TopicCategory),
		ImportanceScore:          normalizeImportanceScore(out.ImportanceScore),
		RecommendationReason:     string(out.RecommendationReason),
		ReadingValue:             string(out.ReadingValue),
		PriorityLevel:            string(out.PriorityLevel),
		ContentPolishedMarkdown:  string(out.ContentPolishedMarkdown),
		AnalysisLongformMarkdown: string(out.AnalysisLongformMarkdown),
		BackgroundContext:        string(out.BackgroundContext),
		ImpactAnalysis:           string(out.ImpactAnalysis),
		DebatePoints:             []string(out.DebatePoints),
		TargetAudience:           string(out.TargetAudience),
		PublishSuggestion:        string(out.PublishSuggestion),
		SuggestionReason:         string(out.SuggestionReason),
		SuggestedChannels:        []string(out.SuggestedChannels),
		SuggestedTags:            []string(out.SuggestedTags),
		SuggestedCategories:      []string(out.SuggestedCategories),
	}, nil
}

type normalizedString string

func (s *normalizedString) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*s = normalizedString(normalizeFlexibleStringValue(value))
	return nil
}

type normalizedStringList []string

func (s *normalizedStringList) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*s = normalizedStringList(normalizeFlexibleStringListValue(value))
	return nil
}

type normalizedPublishSuggestion string

func (s *normalizedPublishSuggestion) UnmarshalJSON(data []byte) error {
	var boolValue bool
	if err := json.Unmarshal(data, &boolValue); err == nil {
		if boolValue {
			*s = normalizedPublishSuggestion("suggested")
		} else {
			*s = normalizedPublishSuggestion("draft")
		}
		return nil
	}

	var value normalizedString
	if err := value.UnmarshalJSON(data); err != nil {
		return err
	}
	*s = normalizedPublishSuggestion(value)
	return nil
}

func normalizeFlexibleStringValue(value any) string {
	switch cast := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(cast)
	case bool:
		return strconv.FormatBool(cast)
	case float64:
		if math.Trunc(cast) == cast {
			return strconv.FormatInt(int64(cast), 10)
		}
		return strconv.FormatFloat(cast, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(cast))
		for _, item := range cast {
			text := normalizeFlexibleStringValue(item)
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		if len(cast) == 0 {
			return ""
		}
		keys := make([]string, 0, len(cast))
		for key := range cast {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			text := normalizeFlexibleStringValue(cast[key])
			if text == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s: %s", key, text))
		}
		return strings.Join(parts, ", ")
	default:
		return strings.TrimSpace(fmt.Sprint(cast))
	}
}

func normalizeFlexibleStringListValue(value any) []string {
	switch cast := value.(type) {
	case nil:
		return nil
	case []any:
		items := make([]string, 0, len(cast))
		for _, item := range cast {
			items = append(items, normalizeFlexibleStringListValue(item)...)
		}
		return items
	case map[string]any:
		if len(cast) == 0 {
			return nil
		}

		keys := make([]string, 0, len(cast))
		for key := range cast {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		items := make([]string, 0, len(keys))
		for _, key := range keys {
			valueText := normalizeFlexibleStringValue(cast[key])
			if valueText == "" {
				continue
			}
			items = append(items, fmt.Sprintf("%s: %s", key, valueText))
		}
		if len(items) == 0 {
			return nil
		}
		return items
	default:
		return splitFlexibleListText(normalizeFlexibleStringValue(cast))
	}
}

func splitFlexibleListText(text string) []string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return nil
	}

	rawParts := []string{text}
	switch {
	case strings.Contains(text, "\n"):
		rawParts = strings.Split(text, "\n")
	case shouldSplitFlexibleListByDelimiter(text):
		replacer := strings.NewReplacer("，", ",", "；", ";", "、", ",", "|", ",")
		normalized := replacer.Replace(text)
		normalized = strings.ReplaceAll(normalized, ";", ",")
		rawParts = strings.Split(normalized, ",")
	}

	items := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		item := trimFlexibleListItem(part)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func shouldSplitFlexibleListByDelimiter(text string) bool {
	if !strings.ContainsAny(text, ",，;；、|") {
		return false
	}
	return !strings.ContainsAny(text, "。！？!?")
}

func trimFlexibleListItem(text string) string {
	item := strings.TrimSpace(text)
	if item == "" {
		return ""
	}

	for _, prefix := range []string{"- ", "* ", "• ", "· "} {
		if strings.HasPrefix(item, prefix) {
			item = strings.TrimSpace(strings.TrimPrefix(item, prefix))
			break
		}
	}

	if idx := strings.IndexAny(item, ".)、"); idx > 0 && isDigits(item[:idx]) {
		item = strings.TrimSpace(item[idx+1:])
	}

	return strings.TrimSpace(item)
}

func isDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, char := range text {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func generateDossierStructuredJSON(ctx context.Context, chat ChatInvoker, prompt string) (string, error) {
	if structured, ok := chat.(structuredJSONDossierChatInvoker); ok {
		return structured.GenerateStructuredJSON(ctx, prompt)
	}
	return chat.Generate(ctx, prompt)
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
