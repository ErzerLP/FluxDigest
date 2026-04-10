package worker

import (
	"github.com/hibiken/asynq"

	asynqtask "rss-platform/internal/task/asynq"
)

// ServerConfig 描述 worker server 的最小配置。
type ServerConfig struct {
	Concurrency int
	Queues      map[string]int
}

// NewServer 创建最小可用的 asynq server。
func NewServer(redisOpt asynq.RedisConnOpt, cfg ServerConfig) *asynq.Server {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	return asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues:      cfg.Queues,
	})
}

// NewServeMux 创建并注册文章处理 handler。
func NewServeMux(articleHandler *asynqtask.ArticleProcessingHandler) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	if articleHandler != nil {
		mux.Handle(asynqtask.TypeProcessArticle, articleHandler.Handler())
	}

	return mux
}
