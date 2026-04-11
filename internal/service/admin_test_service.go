package service

import (
	"context"
	"errors"
	"time"
)

var errAdminTestLLMRequired = errors.New("llm connectivity checker is required")

// LLMTestDraft 表示 LLM 连接测试的最小输入。
type LLMTestDraft struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
}

// ConnectivityTestResult 表示连通性测试结果。
type ConnectivityTestResult struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

// LLMConnectivityChecker 定义 LLM 连通性检测能力。
type LLMConnectivityChecker interface {
	Check(ctx context.Context, draft LLMTestDraft) (time.Duration, error)
}

// ConnectivityChecker 定义通用连通性检测能力。
type ConnectivityChecker interface {
	Check(ctx context.Context) (time.Duration, error)
}

// JobRunWriter 定义写入作业记录所需的最小能力。
type JobRunWriter interface {
	Create(ctx context.Context, record JobRunRecord) error
}

// AdminTestService 负责管理后台的连通性测试。
type AdminTestService struct {
	llm       LLMConnectivityChecker
	miniflux  ConnectivityChecker
	publisher ConnectivityChecker
	jobs      JobRunWriter
}

// NewAdminTestService 创建 AdminTestService。
func NewAdminTestService(llm LLMConnectivityChecker, miniflux ConnectivityChecker, publisher ConnectivityChecker, jobs JobRunWriter) *AdminTestService {
	return &AdminTestService{
		llm:       llm,
		miniflux:  miniflux,
		publisher: publisher,
		jobs:      jobs,
	}
}

// TestLLM 运行 LLM 连通性测试并记录结果。
func (s *AdminTestService) TestLLM(ctx context.Context, draft LLMTestDraft) (ConnectivityTestResult, error) {
	if s == nil || s.llm == nil {
		return ConnectivityTestResult{}, errAdminTestLLMRequired
	}

	latency, err := s.llm.Check(ctx, draft)
	result := ConnectivityTestResult{Status: "ok", Message: "connection succeeded", LatencyMS: latency.Milliseconds()}
	if err != nil {
		result.Status = "error"
		result.Message = err.Error()
	}

	now := time.Now()
	if s.jobs != nil {
		_ = s.jobs.Create(ctx, JobRunRecord{
			JobType:     "llm_test",
			Status:      result.Status,
			Detail:      map[string]any{"message": result.Message, "latency_ms": result.LatencyMS},
			RequestedAt: now,
			FinishedAt:  now,
		})
	}

	if err != nil {
		return result, err
	}
	return result, nil
}
