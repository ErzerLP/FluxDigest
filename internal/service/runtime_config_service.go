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

type activeProfileSnapshot struct {
	version profile.Version
	payload map[string]any
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

	llmProfile, err := s.activeProfile(ctx, profile.TypeLLM)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if s.shouldUseExplicitStringOverride(llmProfile.version, "base_url", llmProfile.payload) {
		snapshot.LLM.BaseURL = stringValue(llmProfile.payload, "base_url")
	} else if value := strings.TrimSpace(stringValue(llmProfile.payload, "base_url")); value != "" {
		snapshot.LLM.BaseURL = value
	}
	if s.shouldUseExplicitStringOverride(llmProfile.version, "api_key", llmProfile.payload) {
		snapshot.LLM.APIKey = stringValue(llmProfile.payload, "api_key")
	} else if value := strings.TrimSpace(stringValue(llmProfile.payload, "api_key")); value != "" {
		snapshot.LLM.APIKey = value
	}
	if s.shouldUseExplicitStringOverride(llmProfile.version, "model", llmProfile.payload) {
		snapshot.LLM.Model = stringValue(llmProfile.payload, "model")
	} else if value := strings.TrimSpace(stringValue(llmProfile.payload, "model")); value != "" {
		snapshot.LLM.Model = value
	}

	schedulerProfile, err := s.activeProfile(ctx, profile.TypeScheduler)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if value, ok := boolValue(schedulerProfile.payload, "schedule_enabled"); ok {
		snapshot.Scheduler.Enabled = value
	}
	if value := strings.TrimSpace(stringValue(schedulerProfile.payload, "schedule_time")); value != "" {
		snapshot.Scheduler.ScheduleTime = value
	}
	if value := strings.TrimSpace(stringValue(schedulerProfile.payload, "timezone")); value != "" {
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

func (s *RuntimeConfigService) activeProfile(ctx context.Context, profileType string) (activeProfileSnapshot, error) {
	if s == nil || s.repo == nil {
		return activeProfileSnapshot{payload: map[string]any{}}, nil
	}

	version, err := s.repo.GetActive(ctx, profileType)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return activeProfileSnapshot{payload: map[string]any{}}, nil
		}
		return activeProfileSnapshot{}, err
	}

	payload := map[string]any{}
	if len(version.PayloadJSON) == 0 {
		return activeProfileSnapshot{version: version, payload: payload}, nil
	}
	if err := json.Unmarshal(version.PayloadJSON, &payload); err != nil {
		return activeProfileSnapshot{}, err
	}

	return activeProfileSnapshot{version: version, payload: payload}, nil
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

func (s *RuntimeConfigService) shouldUseExplicitStringOverride(version profile.Version, key string, payload map[string]any) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	_, isString := value.(string)
	if !isString {
		return false
	}
	return !isDefaultSeedProfile(version)
}

func isDefaultSeedProfile(version profile.Version) bool {
	return version.Name == "default-"+version.ProfileType
}
