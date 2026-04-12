package asynqtask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// ProcessArticleFunc 定义异步任务处理所需的最小回调能力。
type ProcessArticleFunc func(ctx context.Context, articleID string) error

// DailyDigestFunc 定义日报任务处理所需的最小回调能力。
// 具体执行时刻由回调内部自行决定并注入 runtime service。
type DailyDigestFunc func(ctx context.Context, payload DailyDigestPayload) error

// ArticleReprocessFunc 定义单篇重跑任务处理所需的最小回调能力。
type ArticleReprocessFunc func(ctx context.Context, payload ReprocessArticlePayload) error

// ArticleProcessingHandler 负责消费文章处理任务。
type ArticleProcessingHandler struct {
	process ProcessArticleFunc
}

// DailyDigestHandler 负责消费日报任务。
type DailyDigestHandler struct {
	process DailyDigestFunc
}

// ArticleReprocessHandler 负责消费单篇重跑任务。
type ArticleReprocessHandler struct {
	process ArticleReprocessFunc
}

// NewArticleProcessingHandler 创建文章处理 handler。
func NewArticleProcessingHandler(process ProcessArticleFunc) *ArticleProcessingHandler {
	return &ArticleProcessingHandler{process: process}
}

// NewDailyDigestHandler 创建日报任务 handler。
func NewDailyDigestHandler(process DailyDigestFunc) *DailyDigestHandler {
	return &DailyDigestHandler{process: process}
}

// NewArticleReprocessHandler 创建单篇重跑 handler。
func NewArticleReprocessHandler(process ArticleReprocessFunc) *ArticleReprocessHandler {
	return &ArticleReprocessHandler{process: process}
}

// Handler 返回可注册到 asynq 的 handler。
func (h *ArticleProcessingHandler) Handler() asynq.Handler {
	return asynq.HandlerFunc(h.ProcessTask)
}

// Handler 返回可注册到 asynq 的日报 handler。
func (h *DailyDigestHandler) Handler() asynq.Handler {
	return asynq.HandlerFunc(h.ProcessTask)
}

// Handler 返回可注册到 asynq 的单篇重跑 handler。
func (h *ArticleReprocessHandler) Handler() asynq.Handler {
	return asynq.HandlerFunc(h.ProcessTask)
}

// ProcessTask 解包任务并转交文章 ID 给回调。
func (h *ArticleProcessingHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != TypeProcessArticle {
		return fmt.Errorf("unexpected task type %q", task.Type())
	}

	var payload ProcessArticlePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	if h.process == nil {
		return nil
	}

	return h.process(ctx, payload.ArticleID)
}

// ProcessTask 解包日报任务并转交 payload 给回调。
func (h *DailyDigestHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != TypeDailyDigest {
		return fmt.Errorf("unexpected task type %q", task.Type())
	}

	var payload DailyDigestPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	if h.process == nil {
		return nil
	}

	return h.process(ctx, payload)
}

// ProcessTask 解包单篇重跑任务并转交 payload 给回调。
func (h *ArticleReprocessHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != TypeReprocessArticle {
		return fmt.Errorf("unexpected task type %q", task.Type())
	}

	var payload ReprocessArticlePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	if h.process == nil {
		return nil
	}

	return h.process(ctx, payload)
}
