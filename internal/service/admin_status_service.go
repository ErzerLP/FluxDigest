package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var errAdminStatusSnapshotRequired = errors.New("admin config snapshot is required")

// AdminStatusConfigReader 定义读取管理员配置快照所需的最小能力。
type AdminStatusConfigReader interface {
	GetSnapshot(ctx context.Context) (AdminConfigSnapshot, error)
}

// JobRunLatestFinder 定义作业最新记录查询所需的最小能力。
type JobRunLatestFinder interface {
	LatestByType(ctx context.Context, jobType string) (JobRunRecord, error)
}

// DigestLatestFinder 定义最新日报查询所需的最小能力。
type DigestLatestFinder interface {
	LatestDigest(ctx context.Context) (DigestView, error)
}

// AdminStatusView 表示管理后台状态视图。
type AdminStatusView struct {
	System       SystemStatusView      `json:"system"`
	Integrations IntegrationStatusView `json:"integrations"`
	Runtime      RuntimeStatusView     `json:"runtime"`
}

// SystemStatusView 表示系统组件状态。
type SystemStatusView struct {
	API   string `json:"api"`
	DB    string `json:"db"`
	Redis string `json:"redis"`
}

// IntegrationStatusView 表示集成连接状态。
type IntegrationStatusView struct {
	LLM       IntegrationState `json:"llm"`
	Miniflux  IntegrationState `json:"miniflux"`
	Publisher IntegrationState `json:"publisher"`
}

// IntegrationState 表示单个集成状态。
type IntegrationState struct {
	Configured     bool   `json:"configured"`
	LastTestStatus string `json:"last_test_status"`
	LastTestAt     string `json:"last_test_at"`
}

// RuntimeStatusView 表示运行时状态。
type RuntimeStatusView struct {
	LatestDigestDate   string `json:"latest_digest_date"`
	LatestDigestStatus string `json:"latest_digest_status"`
	LatestJobStatus    string `json:"latest_job_status"`
}

// AdminStatusService 负责构建管理后台状态视图。
type AdminStatusService struct {
	configs AdminStatusConfigReader
	jobs    JobRunLatestFinder
	digests DigestLatestFinder
}

// NewAdminStatusService 创建 AdminStatusService。
func NewAdminStatusService(configs AdminStatusConfigReader, jobs JobRunLatestFinder) *AdminStatusService {
	return NewAdminStatusServiceWithDigest(configs, jobs, nil)
}

// NewAdminStatusServiceWithDigest 创建 AdminStatusService，允许注入 digest 查询能力。
func NewAdminStatusServiceWithDigest(configs AdminStatusConfigReader, jobs JobRunLatestFinder, digests DigestLatestFinder) *AdminStatusService {
	return &AdminStatusService{configs: configs, jobs: jobs, digests: digests}
}

// GetStatus 返回后台状态视图。
func (s *AdminStatusService) GetStatus(ctx context.Context) (AdminStatusView, error) {
	if s == nil || s.configs == nil {
		return AdminStatusView{}, errAdminStatusSnapshotRequired
	}

	snapshot, err := s.configs.GetSnapshot(ctx)
	if err != nil {
		return AdminStatusView{}, err
	}

	latestRun := JobRunRecord{}
	latestLLMTest := JobRunRecord{}
	if s.jobs != nil {
		latestRun, err = s.jobs.LatestByType(ctx, "daily_digest_run")
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminStatusView{}, err
		}
		latestLLMTest, err = s.jobs.LatestByType(ctx, "llm_test")
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminStatusView{}, err
		}
	}

	latestDigest := DigestView{}
	if s.digests != nil {
		latestDigest, err = s.digests.LatestDigest(ctx)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminStatusView{}, err
		}
	}

	llmConfigured := snapshot.LLM.BaseURL != "" && snapshot.LLM.APIKey.IsSet

	return AdminStatusView{
		System: SystemStatusView{
			API:   "unknown",
			DB:    "unknown",
			Redis: "unknown",
		},
		Integrations: IntegrationStatusView{
			LLM: IntegrationState{
				Configured:     llmConfigured,
				LastTestStatus: latestLLMTest.Status,
				LastTestAt:     formatRFC3339(latestLLMTest.FinishedAt),
			},
		},
		Runtime: RuntimeStatusView{
			LatestDigestDate:   latestDigest.DigestDate,
			LatestDigestStatus: latestDigest.PublishState,
			LatestJobStatus:    latestRun.Status,
		},
	}, nil
}

func formatRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
