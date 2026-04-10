package asynqtask

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const (
	TypeProcessArticle = "article.process"
	TypeDailyDigest    = "digest.daily"
)

// ProcessArticlePayload 描述单篇文章处理任务载荷。
type ProcessArticlePayload struct {
	ArticleID string `json:"article_id"`
}

// DailyDigestPayload 描述日报任务载荷。
type DailyDigestPayload struct {
	DigestDate string `json:"digest_date"`
}

// NewProcessArticleTask 构造文章处理任务。
func NewProcessArticleTask(articleID string, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(ProcessArticlePayload{ArticleID: articleID})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeProcessArticle, body, opts...), nil
}

// NewDailyDigestTask 构造日报任务。
func NewDailyDigestTask(digestDate string, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(DailyDigestPayload{DigestDate: digestDate})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeDailyDigest, body, opts...), nil
}
