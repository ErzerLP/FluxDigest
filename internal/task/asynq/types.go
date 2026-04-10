package asynqtask

import (
	"encoding/json"

	"github.com/hibiken/asynq"

	"rss-platform/internal/domain/article"
)

const TypeProcessArticle = "article.process"

// ProcessArticlePayload 描述单篇文章处理任务载荷。
type ProcessArticlePayload struct {
	Article article.SourceArticle `json:"article"`
}

// NewProcessArticleTask 构造文章处理任务。
func NewProcessArticleTask(payload ProcessArticlePayload, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeProcessArticle, body, opts...), nil
}
