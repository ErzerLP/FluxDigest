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

func TestDossierBuilderBuildAcceptsTargetAudienceArray(t *testing.T) {
	chat := &dossierChatStub{response: "```json\n{\"title_translated\":\"模型新闻\",\"target_audience\":[\"工程师\",\"产品经理\"]}\n```"}
	builder := llm.NewDossierBuilderFromTemplateText(chat, "生成 dossier")

	out, err := builder.Build(context.Background(), llm.DossierBuildInput{
		Article:    article.SourceArticle{ID: "art-1", Title: "Model News"},
		Processing: postgres.ProcessedArticleRecord{ID: "proc-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TargetAudience != "工程师, 产品经理" {
		t.Fatalf("want normalized target audience got %q", out.TargetAudience)
	}
}

func TestDossierBuilderBuildAcceptsReadingValueArray(t *testing.T) {
	chat := &dossierChatStub{response: "```json\n{\"reading_value\":[\"快速\",\"高质量\"]}\n```"}
	builder := llm.NewDossierBuilderFromTemplateText(chat, "生成 dossier")

	out, err := builder.Build(context.Background(), llm.DossierBuildInput{
		Article:    article.SourceArticle{ID: "art-1", Title: "Model News"},
		Processing: postgres.ProcessedArticleRecord{ID: "proc-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ReadingValue != "快速, 高质量" {
		t.Fatalf("want normalized reading value array got %q", out.ReadingValue)
	}
}

func TestDossierBuilderBuildAcceptsReadingValueObject(t *testing.T) {
	chat := &dossierChatStub{response: "```json\n{\"reading_value\":{\"duration\":\"3分钟\",\"level\":\"high\"}}\n```"}
	builder := llm.NewDossierBuilderFromTemplateText(chat, "生成 dossier")

	out, err := builder.Build(context.Background(), llm.DossierBuildInput{
		Article:    article.SourceArticle{ID: "art-1", Title: "Model News"},
		Processing: postgres.ProcessedArticleRecord{ID: "proc-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ReadingValue != "duration: 3分钟, level: high" {
		t.Fatalf("want normalized reading value object got %q", out.ReadingValue)
	}
}

func TestDossierBuilderBuildAcceptsPublishSuggestionBool(t *testing.T) {
	chat := &dossierChatStub{response: "```json\n{\"publish_suggestion\":true}\n```"}
	builder := llm.NewDossierBuilderFromTemplateText(chat, "生成 dossier")

	out, err := builder.Build(context.Background(), llm.DossierBuildInput{
		Article:    article.SourceArticle{ID: "art-1", Title: "Model News"},
		Processing: postgres.ProcessedArticleRecord{ID: "proc-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.PublishSuggestion != "suggested" {
		t.Fatalf("want normalized publish suggestion got %q", out.PublishSuggestion)
	}
}

func TestDossierBuilderBuildAcceptsFlexibleListFields(t *testing.T) {
	chat := &dossierChatStub{response: "```json\n{\"key_points\":\"- 要点一\\n- 要点二\",\"debate_points\":\"- 观点一\\n- 观点二\",\"suggested_channels\":{\"holo\":\"primary, backup\",\"blog\":\"secondary\"},\"suggested_tags\":[\"AI\",[\"LLM\",\"RSS\"]],\"suggested_categories\":\"技术, 资讯\"}\n```"}
	builder := llm.NewDossierBuilderFromTemplateText(chat, "生成 dossier")

	out, err := builder.Build(context.Background(), llm.DossierBuildInput{
		Article:    article.SourceArticle{ID: "art-1", Title: "Model News"},
		Processing: postgres.ProcessedArticleRecord{ID: "proc-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.DebatePoints) != 2 || out.DebatePoints[0] != "观点一" || out.DebatePoints[1] != "观点二" {
		t.Fatalf("want normalized debate points got %#v", out.DebatePoints)
	}
	if len(out.KeyPoints) != 2 || out.KeyPoints[0] != "要点一" || out.KeyPoints[1] != "要点二" {
		t.Fatalf("want normalized key points got %#v", out.KeyPoints)
	}
	if len(out.SuggestedChannels) != 2 || out.SuggestedChannels[0] != "blog: secondary" || out.SuggestedChannels[1] != "holo: primary, backup" {
		t.Fatalf("want normalized suggested channels got %#v", out.SuggestedChannels)
	}
	if len(out.SuggestedTags) != 3 || out.SuggestedTags[0] != "AI" || out.SuggestedTags[1] != "LLM" || out.SuggestedTags[2] != "RSS" {
		t.Fatalf("want normalized suggested tags got %#v", out.SuggestedTags)
	}
	if len(out.SuggestedCategories) != 2 || out.SuggestedCategories[0] != "技术" || out.SuggestedCategories[1] != "资讯" {
		t.Fatalf("want normalized suggested categories got %#v", out.SuggestedCategories)
	}
}
