package llm_test

import (
	"context"
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
