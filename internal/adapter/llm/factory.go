package llm

import (
	"context"
	"time"

	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// FactoryConfig 定义 OpenAI-compatible ChatModel 所需配置。
type FactoryConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// NewChatModel 创建 OpenAI-compatible ChatModel。
func NewChatModel(ctx context.Context, cfg FactoryConfig) (model.BaseChatModel, error) {
	return openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	})
}
