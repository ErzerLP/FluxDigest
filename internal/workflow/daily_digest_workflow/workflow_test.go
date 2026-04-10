package daily_digest_workflow_test

import (
	"context"
	"strings"
	"testing"

	renderpkg "rss-platform/internal/render"
	workflow "rss-platform/internal/workflow/daily_digest_workflow"
)

type plannerStub struct{}

func (plannerStub) Plan(_ context.Context, items []workflow.CandidateArticle) (workflow.Plan, error) {
	return workflow.Plan{
		Title:       "今日 AI 日报",
		Subtitle:    "聚焦模型与产品动态",
		OpeningNote: "以下为今日重点。",
		Sections: []workflow.Section{{
			Name:  "重点速览",
			Items: []string{items[0].Title},
		}},
	}, nil
}

type rendererStub struct{}

func (rendererStub) Render(plan workflow.Plan, _ []workflow.CandidateArticle) (string, string, error) {
	return "# " + plan.Title, "<h1>" + plan.Title + "</h1>", nil
}

func TestWorkflowGenerateDigest(t *testing.T) {
	wf := workflow.New(plannerStub{}, rendererStub{})

	digest, err := wf.Run(context.Background(), []workflow.CandidateArticle{{ID: "art-1", Title: "Model News"}})
	if err != nil {
		t.Fatal(err)
	}
	if digest.Title != "今日 AI 日报" {
		t.Fatalf("want 今日 AI 日报 got %s", digest.Title)
	}
}

func TestDigestRendererRenderOutputsMarkdownAndHTML(t *testing.T) {
	r := renderpkg.NewDigestRenderer()

	markdown, html, err := r.Render(workflow.Plan{
		Title:       "今日 AI 日报",
		Subtitle:    "聚焦模型与产品动态",
		OpeningNote: "以下为今日重点。",
		Sections: []workflow.Section{{
			Name:  "重点速览",
			Items: []string{"Model News"},
		}},
	}, []workflow.CandidateArticle{{ID: "art-1", Title: "Model News", CoreSummary: "Summary"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "# 今日 AI 日报") {
		t.Fatalf("expected markdown title, got %s", markdown)
	}
	if !strings.Contains(html, "<h1>今日 AI 日报</h1>") {
		t.Fatalf("expected html title, got %s", html)
	}
}
