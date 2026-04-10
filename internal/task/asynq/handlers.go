package asynqtask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	workflow "rss-platform/internal/workflow/article_processing_workflow"
)

// ArticleWorkflow 定义异步任务所需的工作流能力。
type ArticleWorkflow interface {
	Run(ctx context.Context, input workflow.Input) (workflow.ProcessedArticle, error)
}

// ArticleProcessingHandler 负责消费文章处理任务。
type ArticleProcessingHandler struct {
	workflow ArticleWorkflow
}

// NewArticleProcessingHandler 创建文章处理 handler。
func NewArticleProcessingHandler(workflow ArticleWorkflow) *ArticleProcessingHandler {
	return &ArticleProcessingHandler{workflow: workflow}
}

// Handler 返回可注册到 asynq 的 handler。
func (h *ArticleProcessingHandler) Handler() asynq.Handler {
	return asynq.HandlerFunc(h.ProcessTask)
}

// ProcessTask 执行单篇文章处理工作流。
func (h *ArticleProcessingHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != TypeProcessArticle {
		return fmt.Errorf("unexpected task type %q", task.Type())
	}

	var payload ProcessArticlePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	_, err := h.workflow.Run(ctx, workflow.Input{Article: payload.Article})
	return err
}
