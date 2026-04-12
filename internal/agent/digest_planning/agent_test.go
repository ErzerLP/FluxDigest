package digest_planning_test

import (
	"context"
	"strings"
	"testing"

	"rss-platform/internal/agent/digest_planning"
	domaindigest "rss-platform/internal/domain/digest"
)

func TestAgentPlanBuildsStructuredJSONPrompt(t *testing.T) {
	runner := &promptRunnerStub{
		response: `{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[{"article_id":"art-1","title":"模型新闻","core_summary":"核心总结"}]}]}`,
	}
	agent := digest_planning.New(digest_planning.NewOpenAIRunner(runner))

	_, err := agent.Plan(context.Background(), []domaindigest.CandidateArticle{{
		ID:          "art-1",
		Title:       "模型新闻",
		CoreSummary: "核心总结",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.prompts) != 1 {
		t.Fatalf("want 1 prompt got %d", len(runner.prompts))
	}

	prompt := runner.prompts[0]
	for _, fragment := range []string{`"title"`, `"opening_note"`, `"sections"`, `"article_id"`, `"core_summary"`, `"articles"`} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt should contain %s, got %s", fragment, prompt)
		}
	}
}
