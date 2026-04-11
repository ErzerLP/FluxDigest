package service

import (
	"context"
	"encoding/json"
	"errors"

	"rss-platform/internal/domain/profile"
)

type adminConfigProfileRepo interface {
	Create(ctx context.Context, v profile.Version) error
	GetActive(ctx context.Context, profileType string) (profile.Version, error)
}

var errAdminConfigRepoMissing = errors.New("admin config repo is required")

// AdminConfigService 负责读取与更新管理员配置。
type AdminConfigService struct {
	repo adminConfigProfileRepo
}

// NewAdminConfigService 创建 AdminConfigService。
func NewAdminConfigService(repo adminConfigProfileRepo) *AdminConfigService {
	return &AdminConfigService{repo: repo}
}

type SecretMode string

const (
	SecretModeKeep    SecretMode = "keep"
	SecretModeReplace SecretMode = "replace"
	SecretModeClear   SecretMode = "clear"
)

type SecretInput struct {
	Mode  SecretMode `json:"mode"`
	Value string     `json:"value,omitempty"`
}

type SecretView struct {
	IsSet       bool   `json:"is_set"`
	MaskedValue string `json:"masked_value,omitempty"`
}

type AdminConfigSnapshot struct {
	LLM LLMConfigView `json:"llm"`
}

type LLMConfigView struct {
	BaseURL string     `json:"base_url"`
	Model   string     `json:"model"`
	APIKey  SecretView `json:"api_key"`
}

type UpdateLLMConfigInput struct {
	BaseURL string      `json:"base_url"`
	Model   string      `json:"model"`
	APIKey  SecretInput `json:"api_key"`
}

// GetSnapshot 返回管理员配置快照。
func (s *AdminConfigService) GetSnapshot(ctx context.Context) (AdminConfigSnapshot, error) {
	payload, _, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil {
		return AdminConfigSnapshot{}, err
	}

	return AdminConfigSnapshot{
		LLM: LLMConfigView{
			BaseURL: stringValue(payload, "base_url"),
			Model:   stringValue(payload, "model"),
			APIKey:  maskSecret(stringValue(payload, "api_key")),
		},
	}, nil
}

// UpdateLLM 更新 LLM 配置。
func (s *AdminConfigService) UpdateLLM(ctx context.Context, input UpdateLLMConfigInput) (profile.Version, error) {
	currentPayload, currentVersion, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil {
		return profile.Version{}, err
	}

	payload := map[string]any{
		"base_url": input.BaseURL,
		"model":    input.Model,
	}

	applySecret(payload, currentPayload, "api_key", input.APIKey)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return profile.Version{}, err
	}

	version := profile.Version{
		ProfileType: profile.TypeLLM,
		Name:        "admin-llm",
		Version:     nextVersion(currentVersion),
		IsActive:    true,
		PayloadJSON: payloadJSON,
	}

	if err := s.repo.Create(ctx, version); err != nil {
		return profile.Version{}, err
	}

	return version, nil
}

func (s *AdminConfigService) loadProfile(ctx context.Context, profileType string) (map[string]any, int, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errAdminConfigRepoMissing
	}

	version, err := s.repo.GetActive(ctx, profileType)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return map[string]any{}, 0, nil
		}
		return nil, 0, err
	}

	payload := map[string]any{}
	if len(version.PayloadJSON) > 0 {
		if err := json.Unmarshal(version.PayloadJSON, &payload); err != nil {
			return nil, 0, err
		}
	}

	return payload, version.Version, nil
}

func maskSecret(value string) SecretView {
	if value == "" {
		return SecretView{}
	}
	if len(value) <= 4 {
		return SecretView{IsSet: true, MaskedValue: "****"}
	}
	return SecretView{IsSet: true, MaskedValue: value[:4] + "****"}
}

func applySecret(payload map[string]any, current map[string]any, key string, input SecretInput) {
	switch input.Mode {
	case SecretModeReplace:
		if input.Value != "" {
			payload[key] = input.Value
		}
	case SecretModeClear:
		payload[key] = ""
	default:
		if existing, ok := current[key].(string); ok {
			payload[key] = existing
		}
	}
}

func stringValue(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	if value, ok := payload[key]; ok {
		if cast, ok := value.(string); ok {
			return cast
		}
	}
	return ""
}

func nextVersion(current int) int {
	if current <= 0 {
		return 1
	}
	return current + 1
}
