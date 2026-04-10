package article_processing_workflow_test

import (
	"context"
	"testing"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/processing"
	workflow "rss-platform/internal/workflow/article_processing_workflow"
)

type processingStub struct{}

func (processingStub) ProcessArticle(_ context.Context, input article.SourceArticle) (processing.ProcessedArticle, error) {
	return processing.ProcessedArticle{
		Article:     input,
		Analysis:    processing.Analysis{TopicCategory: "AI"},
		Translation: processing.Translation{TitleTranslated: input.Title + " (ZH)"},
	}, nil
}

func TestWorkflowRunReturnsProcessedArticle(t *testing.T) {
	wf := workflow.New(processingStub{})

	out, err := wf.Run(context.Background(), workflow.Input{Article: article.SourceArticle{ID: "art-1", Title: "Original"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Category != "AI" {
		t.Fatalf("want AI got %s", out.Category)
	}
}
