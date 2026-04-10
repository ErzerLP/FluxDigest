package daily_digest_workflow_test

import (
	"context"
	"strings"
	"testing"

	domaindigest "rss-platform/internal/domain/digest"
	renderpkg "rss-platform/internal/render"
	workflow "rss-platform/internal/workflow/daily_digest_workflow"
)

type plannerStub struct{}

func (plannerStub) Plan(_ context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
	return domaindigest.Plan{
		Title:       "今日 AI 日报",
		Subtitle:    "聚焦模型与产品动态",
		OpeningNote: "以下为今日重点。",
		Sections: []domaindigest.Section{{
			Name: "重点速览",
			Items: []domaindigest.SectionItem{{
				ArticleID:   items[0].ID,
				Title:       items[0].Title,
				CoreSummary: items[0].CoreSummary,
			}},
		}},
	}, nil
}

func TestWorkflowGenerateDigestOnlyRendersPlannedItems(t *testing.T) {
	wf := workflow.New(plannerStub{}, renderpkg.NewDigestRenderer())

	digest, err := wf.Run(context.Background(), []domaindigest.CandidateArticle{
		{ID: "art-1", Title: "Model News", CoreSummary: "Selected"},
		{ID: "art-2", Title: "Ignored News", CoreSummary: "Should not appear"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if digest.Title != "今日 AI 日报" {
		t.Fatalf("want 今日 AI 日报 got %s", digest.Title)
	}
	if !strings.Contains(digest.ContentMarkdown, "Model News") {
		t.Fatalf("expected selected article in digest, got %s", digest.ContentMarkdown)
	}
	if strings.Contains(digest.ContentMarkdown, "Ignored News") {
		t.Fatalf("unexpected unplanned article in digest, got %s", digest.ContentMarkdown)
	}
	if got := digest.Plan.Sections[0].Items[0].ArticleID; got != "art-1" {
		t.Fatalf("want art-1 got %s", got)
	}
}
