package service_test

import (
	"context"
	"reflect"
	"testing"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/service"
)

type llmStub struct {
	calls []string
}

func (s *llmStub) Translate(_ context.Context, input article.SourceArticle) (processing.Translation, error) {
	s.calls = append(s.calls, "translate")
	return processing.Translation{TitleTranslated: input.Title + " (ZH)"}, nil
}

func (s *llmStub) Analyze(_ context.Context, _ article.SourceArticle) (processing.Analysis, error) {
	s.calls = append(s.calls, "analyze")
	return processing.Analysis{TopicCategory: "AI"}, nil
}

func TestProcessingServiceTranslateAndAnalyze(t *testing.T) {
	processor := &llmStub{}
	svc := service.NewProcessingService(processor)

	result, err := svc.ProcessArticle(context.Background(), article.SourceArticle{Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(processor.calls, []string{"translate", "analyze"}) {
		t.Fatalf("want call order [translate analyze] got %v", processor.calls)
	}
	if result.Analysis.TopicCategory != "AI" {
		t.Fatalf("want AI got %s", result.Analysis.TopicCategory)
	}
}
