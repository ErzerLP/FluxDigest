package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/processing"
)

var (
	errChatInvokerRequired    = errors.New("llm chat invoker is required")
	errCoreSummaryRequired    = errors.New("analysis core_summary is required")
	errTopicCategoryRequired  = errors.New("analysis topic_category is required")
	errImportanceScoreInvalid = errors.New("analysis importance_score must be between 0 and 1")
)

type promptTemplate struct {
	path string
	text string
}

// ChatInvoker 定义最小文本生成边界。
type ChatInvoker interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// ArticleProcessor 负责翻译与分析单篇文章。
type ArticleProcessor struct {
	chat                ChatInvoker
	translationTemplate promptTemplate
	analysisTemplate    promptTemplate
}

// NewArticleProcessor 创建文章处理适配器。
func NewArticleProcessor(chat ChatInvoker, translationTemplate, analysisTemplate string) *ArticleProcessor {
	return &ArticleProcessor{
		chat:                chat,
		translationTemplate: promptTemplate{path: translationTemplate},
		analysisTemplate:    promptTemplate{path: analysisTemplate},
	}
}

// NewArticleProcessorFromTemplateText 创建不依赖运行时文件路径的文章处理适配器。
func NewArticleProcessorFromTemplateText(chat ChatInvoker, translationTemplateText, analysisTemplateText string) *ArticleProcessor {
	return &ArticleProcessor{
		chat:                chat,
		translationTemplate: promptTemplate{text: translationTemplateText},
		analysisTemplate:    promptTemplate{text: analysisTemplateText},
	}
}

// Process 顺序执行翻译与分析。
func (p *ArticleProcessor) Process(ctx context.Context, input article.SourceArticle) (processing.ProcessedArticle, error) {
	translation, err := p.Translate(ctx, input)
	if err != nil {
		return processing.ProcessedArticle{}, err
	}

	analysis, err := p.Analyze(ctx, input)
	if err != nil {
		return processing.ProcessedArticle{}, err
	}

	return processing.ProcessedArticle{
		Article:     input,
		Translation: translation,
		Analysis:    analysis,
	}, nil
}

// Translate 调用模型生成结构化翻译结果。
func (p *ArticleProcessor) Translate(ctx context.Context, input article.SourceArticle) (processing.Translation, error) {
	if p == nil || p.chat == nil {
		return processing.Translation{}, errChatInvokerRequired
	}

	prompt, err := buildPrompt(p.translationTemplate, input, `仅输出 JSON：{"title_translated":"","summary_translated":"","content_translated":""}`)
	if err != nil {
		return processing.Translation{}, err
	}

	raw, err := p.chat.Generate(ctx, prompt)
	if err != nil {
		return processing.Translation{}, err
	}

	var out struct {
		TitleTranslated   string `json:"title_translated"`
		SummaryTranslated string `json:"summary_translated"`
		ContentTranslated string `json:"content_translated"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONObject(raw)), &out); err != nil {
		return processing.Translation{}, err
	}
	return processing.Translation{
		TitleTranslated:   out.TitleTranslated,
		SummaryTranslated: out.SummaryTranslated,
		ContentTranslated: out.ContentTranslated,
	}, nil
}

// Analyze 调用模型生成结构化分析结果。
func (p *ArticleProcessor) Analyze(ctx context.Context, input article.SourceArticle) (processing.Analysis, error) {
	if p == nil || p.chat == nil {
		return processing.Analysis{}, errChatInvokerRequired
	}

	prompt, err := buildPrompt(p.analysisTemplate, input, "")
	if err != nil {
		return processing.Analysis{}, err
	}

	raw, err := p.chat.Generate(ctx, prompt)
	if err != nil {
		return processing.Analysis{}, err
	}

	var out struct {
		CoreSummary     string   `json:"core_summary"`
		KeyPoints       []string `json:"key_points"`
		TopicCategory   string   `json:"topic_category"`
		ImportanceScore float64  `json:"importance_score"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONObject(raw)), &out); err != nil {
		return processing.Analysis{}, err
	}
	analysis := processing.Analysis{
		CoreSummary:     out.CoreSummary,
		KeyPoints:       out.KeyPoints,
		TopicCategory:   out.TopicCategory,
		ImportanceScore: normalizeImportanceScore(out.ImportanceScore),
	}
	if err := validateAnalysis(analysis); err != nil {
		return processing.Analysis{}, err
	}
	return analysis, nil
}

func buildPrompt(promptSpec promptTemplate, input article.SourceArticle, schemaHint string) (string, error) {
	templateText, err := loadTemplate(promptSpec)
	if err != nil {
		return "", err
	}

	name := filepath.Base(promptSpec.path)
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
	if schemaHint != "" {
		prompt.WriteString(schemaHint)
		prompt.WriteString("\n")
	}
	prompt.WriteString("输入文章 JSON：")
	prompt.Write(payload)

	return prompt.String(), nil
}

func loadTemplate(promptSpec promptTemplate) (string, error) {
	if promptSpec.text != "" {
		return promptSpec.text, nil
	}

	resolved, err := resolvePath(promptSpec.path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func validateAnalysis(analysis processing.Analysis) error {
	if strings.TrimSpace(analysis.CoreSummary) == "" {
		return errCoreSummaryRequired
	}
	if strings.TrimSpace(analysis.TopicCategory) == "" {
		return errTopicCategoryRequired
	}
	if analysis.ImportanceScore < 0 || analysis.ImportanceScore > 1 {
		return errImportanceScoreInvalid
	}

	return nil
}

func normalizeImportanceScore(score float64) float64 {
	if score > 1 && score <= 10 {
		return score / 10
	}

	return score
}

func resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	current := wd
	for {
		candidate := filepath.Join(current, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("resolve path %s: %w", path, os.ErrNotExist)
}

func normalizeJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		return trimmed[start : end+1]
	}

	return trimmed
}
