package service

import (
	"context"
	"errors"
	"time"
)

var errAdminTestLLMRequired = errors.New("llm connectivity checker is required")
var errAdminTestMinifluxRequired = errors.New("miniflux connectivity checker is required")
var errAdminTestPublishRequired = errors.New("publish connectivity checker is required")

const defaultAdminLLMTestTimeoutMS = 30000
const maxAdminLLMTimeoutMS = 2_147_483_647

// LLMTestDraft 表示 LLM 连接测试的最小输入。
type LLMTestDraft struct {
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
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
	draft.TimeoutMS = normalizeAdminLLMTimeoutMS(draft.TimeoutMS)

	latency, checkErr := s.llm.Check(ctx, draft)
	return s.finishConnectivityTest(ctx, "llm_test", latency, checkErr)
}

// TestMiniflux 运行 Miniflux 连通性测试并记录结果。
func (s *AdminTestService) TestMiniflux(ctx context.Context) (ConnectivityTestResult, error) {
	if s == nil || s.miniflux == nil {
		return ConnectivityTestResult{}, errAdminTestMinifluxRequired
	}

	latency, checkErr := s.miniflux.Check(ctx)
	return s.finishConnectivityTest(ctx, "miniflux_test", latency, checkErr)
}

// TestPublish 运行发布链路连通性测试并记录结果。
func (s *AdminTestService) TestPublish(ctx context.Context) (ConnectivityTestResult, error) {
	if s == nil || s.publisher == nil {
		return ConnectivityTestResult{}, errAdminTestPublishRequired
	}

	latency, checkErr := s.publisher.Check(ctx)
	return s.finishConnectivityTest(ctx, "publish_test", latency, checkErr)
}

func (s *AdminTestService) finishConnectivityTest(ctx context.Context, jobType string, latency time.Duration, checkErr error) (ConnectivityTestResult, error) {
	result := ConnectivityTestResult{Status: "ok", Message: "connection succeeded", LatencyMS: latency.Milliseconds()}
	if checkErr != nil {
		result.Status = "error"
		result.Message = checkErr.Error()
	}

	now := time.Now()
	var persistErr error
	if s != nil && s.jobs != nil {
		persistErr = s.jobs.Create(ctx, JobRunRecord{
			JobType:     jobType,
			Status:      result.Status,
			Detail:      map[string]any{"message": result.Message, "latency_ms": result.LatencyMS},
			RequestedAt: now,
			FinishedAt:  now,
		})
	}

	if checkErr != nil && persistErr != nil {
		return result, errors.Join(checkErr, persistErr)
	}
	if persistErr != nil {
		return result, persistErr
	}
	if checkErr != nil {
		return result, checkErr
	}

	return result, nil
}

func normalizeAdminLLMTimeoutMS(timeoutMS int) int {
	if timeoutMS <= 0 {
		return defaultAdminLLMTestTimeoutMS
	}
	if timeoutMS > maxAdminLLMTimeoutMS {
		return maxAdminLLMTimeoutMS
	}
	return timeoutMS
}
