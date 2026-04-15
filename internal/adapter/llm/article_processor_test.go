package llm_test

import (
	"context"
	"strings"
	"testing"

	"rss-platform/internal/adapter/llm"
	"rss-platform/internal/domain/article"
)

type chatModelStub struct {
	responses []string
	calls     []string
}

func (s *chatModelStub) Generate(_ context.Context, prompt string) (string, error) {
	s.calls = append(s.calls, prompt)
	if len(s.responses) == 0 {
		return "", nil
	}

	out := s.responses[0]
	s.responses = s.responses[1:]
	return out, nil
}

func TestArticleProcessorParsesTranslationAndAnalysisJSON(t *testing.T) {
	model := &chatModelStub{responses: []string{
		`{"title_translated":"标题","summary_translated":"摘要","content_translated":"正文"}`,
		`{"core_summary":"核心总结","key_points":["a"],"topic_category":"AI","importance_score":0.9}`,
	}}
	processor := llm.NewArticleProcessor(model, "configs/prompts/translation.tmpl", "configs/prompts/analysis.tmpl")

	out, err := processor.Process(context.Background(), article.SourceArticle{Title: "Original", ContentText: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Translation.TitleTranslated != "标题" {
		t.Fatalf("want 标题 got %s", out.Translation.TitleTranslated)
	}
	if out.Analysis.TopicCategory != "AI" {
		t.Fatalf("want AI got %s", out.Analysis.TopicCategory)
	}
	if len(model.calls) != 2 {
		t.Fatalf("want 2 calls got %d", len(model.calls))
	}
}

func TestArticleProcessorAnalyzePromptUsesUnifiedSchema(t *testing.T) {
	model := &chatModelStub{responses: []string{
		`{"core_summary":"核心总结","key_points":["a"],"topic_category":"AI","importance_score":0.9}`,
	}}
	processor := llm.NewArticleProcessor(model, "configs/prompts/translation.tmpl", "configs/prompts/analysis.tmpl")

	_, err := processor.Analyze(context.Background(), article.SourceArticle{Title: "Original", ContentText: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(model.calls) != 1 {
		t.Fatalf("want 1 call got %d", len(model.calls))
	}

	prompt := model.calls[0]
	if strings.Contains(prompt, `"risks"`) || strings.Contains(prompt, `"audience"`) {
		t.Fatalf("analysis prompt should not contain legacy schema, got %s", prompt)
	}
	if !strings.Contains(prompt, `"core_summary"`) || !strings.Contains(prompt, `"topic_category"`) || !strings.Contains(prompt, `"importance_score"`) {
		t.Fatalf("analysis prompt should contain unified schema, got %s", prompt)
	}
	if strings.Count(prompt, `"core_summary"`) != 1 {
		t.Fatalf("analysis prompt should contain exactly one schema contract, got %s", prompt)
	}
}

func TestArticleProcessorAnalyzeNormalizesTenPointImportanceScore(t *testing.T) {
	model := &chatModelStub{responses: []string{
		`{"core_summary":"核心总结","key_points":["a"],"topic_category":"AI","importance_score":8}`,
	}}
	processor := llm.NewArticleProcessor(model, "configs/prompts/translation.tmpl", "configs/prompts/analysis.tmpl")

	out, err := processor.Analyze(context.Background(), article.SourceArticle{Title: "Original", ContentText: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ImportanceScore != 0.8 {
		t.Fatalf("want 0.8 got %v", out.ImportanceScore)
	}
}

func TestArticleProcessorAnalyzeRejectsInvalidAnalysisJSON(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{
			name:     "missing core summary",
			response: `{"key_points":["a"],"topic_category":"AI","importance_score":0.9}`,
		},
		{
			name:     "missing topic category",
			response: `{"core_summary":"核心总结","key_points":["a"],"importance_score":0.9}`,
		},
		{
			name:     "negative importance score",
			response: `{"core_summary":"核心总结","key_points":["a"],"topic_category":"AI","importance_score":-0.1}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := &chatModelStub{responses: []string{tc.response}}
			processor := llm.NewArticleProcessor(model, "configs/prompts/translation.tmpl", "configs/prompts/analysis.tmpl")

			_, err := processor.Analyze(context.Background(), article.SourceArticle{Title: "Original", ContentText: "Hello"})
			if err == nil {
				t.Fatal("want validation error")
			}
		})
	}
}
