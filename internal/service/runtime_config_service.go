package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"rss-platform/internal/config"
	"rss-platform/internal/domain/profile"
)

// LLMRuntimeConfig 表示 worker 使用的 LLM 运行时配置。
type LLMRuntimeConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// SchedulerRuntimeConfig 表示 scheduler 使用的运行时配置。
type SchedulerRuntimeConfig struct {
	Enabled      bool   `json:"enabled"`
	ScheduleTime string `json:"schedule_time"`
	Timezone     string `json:"timezone"`
}

// RuntimeSnapshot 表示运行时配置快照。
type RuntimeSnapshot struct {
	LLM       LLMRuntimeConfig       `json:"llm"`
	Scheduler SchedulerRuntimeConfig `json:"scheduler"`
}

// RuntimeConfigService 负责按 DB 优先、配置兜底读取运行时配置。
type RuntimeConfigService struct {
	repo     ProfileRepository
	defaults *config.Config
}

// NewRuntimeConfigService 创建 RuntimeConfigService。
func NewRuntimeConfigService(repo ProfileRepository, defaults *config.Config) *RuntimeConfigService {
	return &RuntimeConfigService{repo: repo, defaults: defaults}
}

// Snapshot 返回运行时配置快照。
func (s *RuntimeConfigService) Snapshot(ctx context.Context) (RuntimeSnapshot, error) {
	snapshot := RuntimeSnapshot{
		LLM: LLMRuntimeConfig{
			BaseURL: s.defaultLLMBaseURL(),
			APIKey:  s.defaultLLMAPIKey(),
			Model:   s.defaultLLMModel(),
		},
		Scheduler: defaultSchedulerRuntimeConfig(),
	}

	llmPayload, err := s.activePayload(ctx, profile.TypeLLM)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if value := strings.TrimSpace(stringValue(llmPayload, "base_url")); value != "" {
		snapshot.LLM.BaseURL = value
	}
	if value := strings.TrimSpace(stringValue(llmPayload, "api_key")); value != "" {
		snapshot.LLM.APIKey = value
	}
	if value := strings.TrimSpace(stringValue(llmPayload, "model")); value != "" {
		snapshot.LLM.Model = value
	}

	schedulerPayload, err := s.activePayload(ctx, profile.TypeScheduler)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if value, ok := boolValue(schedulerPayload, "schedule_enabled"); ok {
		snapshot.Scheduler.Enabled = value
	}
	if value := strings.TrimSpace(stringValue(schedulerPayload, "schedule_time")); value != "" {
		snapshot.Scheduler.ScheduleTime = value
	}
	if value := strings.TrimSpace(stringValue(schedulerPayload, "timezone")); value != "" {
		snapshot.Scheduler.Timezone = value
	}

	return snapshot, nil
}

// LLM 返回 LLM 运行时配置。
func (s *RuntimeConfigService) LLM(ctx context.Context) (LLMRuntimeConfig, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return LLMRuntimeConfig{}, err
	}
	return snapshot.LLM, nil
}

// Scheduler 返回 scheduler 运行时配置。
func (s *RuntimeConfigService) Scheduler(ctx context.Context) (SchedulerRuntimeConfig, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return SchedulerRuntimeConfig{}, err
	}
	return snapshot.Scheduler, nil
}

func (s *RuntimeConfigService) activePayload(ctx context.Context, profileType string) (map[string]any, error) {
	if s == nil || s.repo == nil {
		return map[string]any{}, nil
	}

	version, err := s.repo.GetActive(ctx, profileType)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return map[string]any{}, nil
		}
		return nil, err
	}

	payload := map[string]any{}
	if len(version.PayloadJSON) == 0 {
		return payload, nil
	}
	if err := json.Unmarshal(version.PayloadJSON, &payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func (s *RuntimeConfigService) defaultLLMBaseURL() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return s.defaults.LLM.BaseURL
}

func (s *RuntimeConfigService) defaultLLMAPIKey() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return s.defaults.LLM.APIKey
}

func (s *RuntimeConfigService) defaultLLMModel() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return s.defaults.LLM.Model
}

func defaultSchedulerRuntimeConfig() SchedulerRuntimeConfig {
	return SchedulerRuntimeConfig{
		Enabled:      true,
		ScheduleTime: "07:00",
		Timezone:     "Asia/Shanghai",
	}
}

func boolValue(payload map[string]any, key string) (bool, bool) {
	if payload == nil {
		return false, false
	}
	value, ok := payload[key]
	if !ok {
		return false, false
	}
	cast, ok := value.(bool)
	return cast, ok
}
