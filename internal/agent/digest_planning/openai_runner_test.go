package digest_planning_test

import (
	"context"
	"testing"

	"rss-platform/internal/agent/digest_planning"
)

type promptRunnerStub struct {
	response string
	prompts  []string
}

func (s *promptRunnerStub) Generate(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	return s.response, nil
}

func TestOpenAIRunnerRunParsesPlanJSON(t *testing.T) {
	runner := digest_planning.NewOpenAIRunner(&promptRunnerStub{
		response: `{"title":"今日 AI 日报","subtitle":"聚焦模型动态","opening_note":"以下为重点","sections":[{"name":"重点速览","items":[{"article_id":"art-1","title":"Model News","core_summary":"核心总结"}]}]}`,
	})

	plan, err := runner.Run(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title != "今日 AI 日报" {
		t.Fatalf("want 今日 AI 日报 got %s", plan.Title)
	}
	if got := plan.Sections[0].Items[0].ArticleID; got != "art-1" {
		t.Fatalf("want art-1 got %s", got)
	}
}
