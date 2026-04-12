package llm_test

import (
	"context"
	"testing"

	"rss-platform/internal/adapter/llm"
	"rss-platform/internal/domain/article"
	"rss-platform/internal/repository/postgres"
)

type dossierChatStub struct {
	response string
	prompts  []string
}

func (s *dossierChatStub) Generate(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	return s.response, nil
}

func TestDossierBuilderBuildParsesJSON(t *testing.T) {
	chat := &dossierChatStub{response: "```json\n{\"title_translated\":\"模型新闻\",\"summary_polished\":\"润色摘要\",\"core_summary\":\"核心总结\",\"key_points\":[\"k1\"],\"topic_category\":\"AI\",\"importance_score\":0.9,\"publish_suggestion\":\"suggested\"}\n```"}
	builder := llm.NewDossierBuilderFromTemplateText(chat, "生成 dossier")

	out, err := builder.Build(context.Background(), llm.DossierBuildInput{
		Article: article.SourceArticle{ID: "art-1", Title: "Model News"},
		Processing: postgres.ProcessedArticleRecord{
			ID:              "proc-1",
			TitleTranslated: "模型新闻",
			CoreSummary:     "核心总结",
			KeyPoints:       []string{"k1"},
			TopicCategory:   "AI",
			ImportanceScore: 0.9,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TitleTranslated != "模型新闻" {
		t.Fatalf("want 模型新闻 got %q", out.TitleTranslated)
	}
	if out.PublishSuggestion != "suggested" {
		t.Fatalf("want suggested got %q", out.PublishSuggestion)
	}
}
