package digest_planning_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"rss-platform/internal/agent/digest_planning"
	domaindigest "rss-platform/internal/domain/digest"
)

type runnerStub struct {
	plan domaindigest.Plan
	err  error
}

func (s runnerStub) Run(_ context.Context, _ string) (domaindigest.Plan, error) {
	return s.plan, s.err
}

func TestAgentPlanBuildsStructuredJSONPrompt(t *testing.T) {
	runner := &promptRunnerStub{
		response: `{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[{"dossier_id":"dos-1","article_id":"art-1","title":"模型新闻","core_summary":"核心总结","importance_bucket":"featured","is_featured":true}]}]}`,
	}
	agent := digest_planning.NewWithPrompt(digest_planning.NewOpenAIRunner(runner), "自定义日报提示词")

	_, err := agent.Plan(context.Background(), []domaindigest.CandidateArticle{{
		ID:          "art-1",
		DossierID:   "dos-1",
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
	for _, fragment := range []string{`自定义日报提示词`, `"title"`, `"opening_note"`, `"sections"`, `"dossier_id"`, `"article_id"`, `"core_summary"`, `"importance_bucket"`, `"is_featured"`, `"articles"`} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt should contain %s, got %s", fragment, prompt)
		}
	}
	for _, fragment := range []string{"article_id 与 dossier_id 必须原样复用输入候选值", "不得编造、改写、拼接或猜测"} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt should enforce trace constraints: missing %s", fragment)
		}
	}
}

func TestAgentPlanFallsBackToDeterministicPlanWhenRunnerFails(t *testing.T) {
	agent := digest_planning.NewWithPrompt(runnerStub{err: errors.New("504 Gateway Time-out")}, "自定义日报提示词")

	plan, err := agent.Plan(context.Background(), []domaindigest.CandidateArticle{
		{
			ID:              "art-2",
			DossierID:       "dos-2",
			Title:           "第二条",
			CoreSummary:     "次要总结",
			ImportanceScore: 0.6,
		},
		{
			ID:              "art-1",
			DossierID:       "dos-1",
			Title:           "第一条",
			CoreSummary:     "重点总结",
			ImportanceScore: 0.9,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title == "" {
		t.Fatal("want fallback title")
	}
	if len(plan.Sections) == 0 {
		t.Fatal("want fallback sections")
	}
	if len(plan.Sections[0].Items) == 0 {
		t.Fatal("want fallback section items")
	}
	if got := plan.Sections[0].Items[0].ArticleID; got != "art-1" {
		t.Fatalf("want highest importance first got %s", got)
	}
	if got := plan.Sections[0].Items[0].DossierID; got != "dos-1" {
		t.Fatalf("want dossier trace kept got %s", got)
	}
}

func TestAgentPlanKeepsRunnerErrorWhenNoCandidates(t *testing.T) {
	agent := digest_planning.NewWithPrompt(runnerStub{err: errors.New("504 Gateway Time-out")}, "自定义日报提示词")

	_, err := agent.Plan(context.Background(), nil)
	if err == nil {
		t.Fatal("want runner error when no candidates")
	}
}
