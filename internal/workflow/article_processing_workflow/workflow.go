package article_processing_workflow

import (
	"context"

	"github.com/cloudwego/eino/compose"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/processing"
)

// ProcessingService 定义工作流所依赖的最小处理能力。
type ProcessingService interface {
	ProcessArticle(ctx context.Context, input article.SourceArticle) (processing.ProcessedArticle, error)
}

// Input 表示文章处理工作流输入。
type Input struct {
	Article article.SourceArticle
}

// ProcessedArticle 表示工作流输出。
type ProcessedArticle struct {
	ArticleID   string
	Category    string
	Translation processing.Translation
	Analysis    processing.Analysis
}

// Workflow 封装单篇文章处理工作流。
type Workflow struct {
	runner     compose.Runnable[Input, ProcessedArticle]
	compileErr error
}

// New 创建最小可用的文章处理工作流。
func New(svc ProcessingService) *Workflow {
	wf := compose.NewWorkflow[Input, ProcessedArticle]()
	wf.AddLambdaNode("process_article", compose.InvokableLambda(func(ctx context.Context, input Input) (ProcessedArticle, error) {
		result, err := svc.ProcessArticle(ctx, input.Article)
		if err != nil {
			return ProcessedArticle{}, err
		}

		articleID := result.Article.ID
		if articleID == "" {
			articleID = input.Article.ID
		}

		return ProcessedArticle{
			ArticleID:   articleID,
			Category:    result.Analysis.TopicCategory,
			Translation: result.Translation,
			Analysis:    result.Analysis,
		}, nil
	})).AddInput(compose.START)
	wf.AddEnd("process_article")

	runner, err := wf.Compile(context.Background())
	return &Workflow{runner: runner, compileErr: err}
}

// Run 执行文章处理工作流。
func (w *Workflow) Run(ctx context.Context, input Input) (ProcessedArticle, error) {
	if w.compileErr != nil {
		return ProcessedArticle{}, w.compileErr
	}

	return w.runner.Invoke(ctx, input)
}
