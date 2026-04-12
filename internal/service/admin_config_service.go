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
var errSecretValueRequired = errors.New("secret value required")

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
	BaseURL   string     `json:"base_url"`
	Model     string     `json:"model"`
	TimeoutMS int        `json:"timeout_ms"`
	APIKey    SecretView `json:"api_key"`
}

type UpdateLLMConfigInput struct {
	BaseURL   string      `json:"base_url"`
	Model     string      `json:"model"`
	TimeoutMS int         `json:"timeout_ms"`
	APIKey    SecretInput `json:"api_key"`
}

// GetSnapshot 返回管理员配置快照。
func (s *AdminConfigService) GetSnapshot(ctx context.Context) (AdminConfigSnapshot, error) {
	payload, _, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil {
		return AdminConfigSnapshot{}, err
	}

	return AdminConfigSnapshot{
		LLM: LLMConfigView{
			BaseURL:   stringValue(payload, "base_url"),
			Model:     stringValue(payload, "model"),
			TimeoutMS: resolveLLMTimeoutMS(payload, 0),
			APIKey:    maskSecret(stringValue(payload, "api_key")),
		},
	}, nil
}

// UpdateLLM 更新 LLM 配置。
func (s *AdminConfigService) UpdateLLM(ctx context.Context, input UpdateLLMConfigInput) (profile.Version, error) {
	currentPayload, currentVersion, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil {
		return profile.Version{}, err
	}

	if input.APIKey.Mode == SecretModeReplace && input.APIKey.Value == "" {
		return profile.Version{}, errSecretValueRequired
	}

	payload := clonePayload(currentPayload)
	payload["base_url"] = input.BaseURL
	payload["model"] = input.Model
	payload["timeout_ms"] = resolveLLMTimeoutMS(currentPayload, input.TimeoutMS)

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

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
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

func resolveLLMTimeoutMS(payload map[string]any, value int) int {
	if value > 0 {
		return value
	}

	current := intValue(payload, "timeout_ms")
	if current > 0 {
		return current
	}

	return 30000
}

func intValue(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}

	value, ok := payload[key]
	if !ok {
		return 0
	}

	switch cast := value.(type) {
	case int:
		return cast
	case int32:
		return int(cast)
	case int64:
		return int(cast)
	case float32:
		return int(cast)
	case float64:
		return int(cast)
	default:
		return 0
	}
}

func nextVersion(current int) int {
	if current <= 0 {
		return 1
	}
	return current + 1
}
