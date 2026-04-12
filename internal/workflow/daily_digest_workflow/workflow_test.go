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

type plannerFuncStub func(ctx context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error)

func (f plannerFuncStub) Plan(ctx context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
	return f(ctx, items)
}

func (plannerStub) Plan(_ context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
	return domaindigest.Plan{
		Title:       "今日 AI 日报",
		Subtitle:    "聚焦模型与产品动态",
		OpeningNote: "以下为今日重点。",
		Sections: []domaindigest.Section{{
			Name: "重点速览",
			Items: []domaindigest.SectionItem{{
				DossierID:        items[0].DossierID,
				ArticleID:        items[0].ID,
				Title:            items[0].Title,
				CoreSummary:      items[0].CoreSummary,
				ImportanceBucket: "featured",
				IsFeatured:       true,
			}},
		}},
	}, nil
}

func TestWorkflowGenerateDigestOnlyRendersPlannedItems(t *testing.T) {
	wf := workflow.New(plannerStub{}, renderpkg.NewDigestRenderer())

	digest, err := wf.Run(context.Background(), []domaindigest.CandidateArticle{
		{ID: "art-1", DossierID: "dos-1", Title: "Model News", CoreSummary: "Selected"},
		{ID: "art-2", DossierID: "dos-2", Title: "Ignored News", CoreSummary: "Should not appear"},
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

func TestWorkflowGenerateDigestPreservesDossierTrace(t *testing.T) {
	wf := workflow.New(plannerStub{}, renderpkg.NewDigestRenderer())

	digest, err := wf.Run(context.Background(), []domaindigest.CandidateArticle{{
		ID:                   "art-1",
		DossierID:            "dos-1",
		Title:                "模型新闻",
		CoreSummary:          "核心总结",
		RecommendationReason: "值得重点跟进",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := digest.Plan.Sections[0].Items[0].DossierID; got != "dos-1" {
		t.Fatalf("want dossier trace dos-1 got %q", got)
	}
	if got := digest.Plan.Sections[0].Items[0].ImportanceBucket; got != "featured" {
		t.Fatalf("want featured bucket got %q", got)
	}
	if !digest.Plan.Sections[0].Items[0].IsFeatured {
		t.Fatal("expected featured item")
	}
}

func TestWorkflowGenerateDigestRejectsUnknownArticleTrace(t *testing.T) {
	wf := workflow.New(plannerFuncStub(func(_ context.Context, _ []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
		return domaindigest.Plan{
			Title: "今日 AI 日报",
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					ArticleID:   "art-x",
					Title:       "未知文章",
					CoreSummary: "未知摘要",
				}},
			}},
		}, nil
	}), renderpkg.NewDigestRenderer())

	_, err := wf.Run(context.Background(), []domaindigest.CandidateArticle{{
		ID:        "art-1",
		DossierID: "dos-1",
		Title:     "模型新闻",
	}})
	if err == nil {
		t.Fatal("expected unknown article trace error")
	}
}

func TestWorkflowGenerateDigestRecoversTraceByDossierID(t *testing.T) {
	wf := workflow.New(plannerFuncStub(func(_ context.Context, _ []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
		return domaindigest.Plan{
			Title: "今日 AI 日报",
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					DossierID:   "dos-1",
					ArticleID:   "art-x",
					Title:       "模型新闻",
					CoreSummary: "核心总结",
				}},
			}},
		}, nil
	}), renderpkg.NewDigestRenderer())

	digest, err := wf.Run(context.Background(), []domaindigest.CandidateArticle{{
		ID:        "art-1",
		DossierID: "dos-1",
		Title:     "模型新闻",
	}})
	if err != nil {
		t.Fatal(err)
	}
	item := digest.Plan.Sections[0].Items[0]
	if item.DossierID != "dos-1" {
		t.Fatalf("want dossier dos-1 got %q", item.DossierID)
	}
	if item.ArticleID != "art-1" {
		t.Fatalf("want recovered article_id art-1 got %q", item.ArticleID)
	}
}

func TestWorkflowGenerateDigestRejectsPlannedItemsWhenCandidatesEmpty(t *testing.T) {
	wf := workflow.New(plannerFuncStub(func(_ context.Context, _ []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
		return domaindigest.Plan{
			Title: "今日 AI 日报",
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					ArticleID:   "art-1",
					Title:       "模型新闻",
					CoreSummary: "核心总结",
				}},
			}},
		}, nil
	}), renderpkg.NewDigestRenderer())

	_, err := wf.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when planner outputs items without candidates")
	}
}
