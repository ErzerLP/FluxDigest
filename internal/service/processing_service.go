package service

import (
	"context"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/processing"
)

// ArticleProcessor 定义文章处理所需的最小能力。
type ArticleProcessor interface {
	Translate(ctx context.Context, article article.SourceArticle) (processing.Translation, error)
	Analyze(ctx context.Context, article article.SourceArticle) (processing.Analysis, error)
}

// ProcessingService 负责编排单篇文章处理。
type ProcessingService struct {
	processor ArticleProcessor
}

// NewProcessingService 创建 ProcessingService。
func NewProcessingService(processor ArticleProcessor) *ProcessingService {
	return &ProcessingService{processor: processor}
}

// ProcessArticle 顺序执行翻译和分析，并返回聚合结果。
func (s *ProcessingService) ProcessArticle(ctx context.Context, input article.SourceArticle) (processing.ProcessedArticle, error) {
	translation, err := s.processor.Translate(ctx, input)
	if err != nil {
		return processing.ProcessedArticle{}, err
	}

	analysis, err := s.processor.Analyze(ctx, input)
	if err != nil {
		return processing.ProcessedArticle{}, err
	}

	return processing.ProcessedArticle{
		Article:     input,
		Translation: translation,
		Analysis:    analysis,
	}, nil
}
