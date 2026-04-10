package asynqtask

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const TypeProcessArticle = "article.process"

// ProcessArticlePayload 描述单篇文章处理任务载荷。
type ProcessArticlePayload struct {
	ArticleID string `json:"article_id"`
}

// NewProcessArticleTask 构造文章处理任务。
func NewProcessArticleTask(articleID string, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(ProcessArticlePayload{ArticleID: articleID})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeProcessArticle, body, opts...), nil
}
