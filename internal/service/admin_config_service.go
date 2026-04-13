package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"rss-platform/internal/domain/profile"
	"rss-platform/internal/security"
)

type adminConfigProfileRepo interface {
	Create(ctx context.Context, v profile.Version) error
	GetActive(ctx context.Context, profileType string) (profile.Version, error)
}

type secretValueCipher interface {
	EncryptString(value string) (string, error)
	DecryptString(value string) (string, error)
}

var errAdminConfigRepoMissing = errors.New("admin config repo is required")
var errSecretValueRequired = errors.New("secret value required")
var errSecretCipherRequired = errors.New("secret cipher is required")
var errUnsupportedPublishProvider = errors.New("unsupported publish provider")

// AdminConfigService 负责读取与更新管理员配置。
type AdminConfigService struct {
	repo   adminConfigProfileRepo
	cipher secretValueCipher
}

// NewAdminConfigService 创建 AdminConfigService。
func NewAdminConfigService(repo adminConfigProfileRepo, cipher secretValueCipher) *AdminConfigService {
	return &AdminConfigService{repo: repo, cipher: cipher}
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
	LLM      LLMConfigView      `json:"llm"`
	Miniflux MinifluxConfigView `json:"miniflux"`
	Publish  PublishConfigView  `json:"publish"`
	Prompts  PromptConfigView   `json:"prompts"`
}

type LLMConfigView struct {
	BaseURL   string     `json:"base_url"`
	Model     string     `json:"model"`
	TimeoutMS int        `json:"timeout_ms"`
	APIKey    SecretView `json:"api_key"`
}

type MinifluxConfigView struct {
	BaseURL       string     `json:"base_url"`
	FetchLimit    int        `json:"fetch_limit"`
	LookbackHours int        `json:"lookback_hours"`
	APIToken      SecretView `json:"api_token"`
}

type PublishConfigView struct {
	Provider    string     `json:"provider"`
	HaloBaseURL string     `json:"halo_base_url"`
	HaloToken   SecretView `json:"halo_token"`
	OutputDir   string     `json:"output_dir"`
}

type PromptConfigView struct {
	TargetLanguage    string `json:"target_language,omitempty"`
	TranslationPrompt string `json:"translation_prompt"`
	AnalysisPrompt    string `json:"analysis_prompt"`
	DossierPrompt     string `json:"dossier_prompt"`
	DigestPrompt      string `json:"digest_prompt"`
}

type UpdateLLMConfigInput struct {
	BaseURL   string      `json:"base_url"`
	Model     string      `json:"model"`
	TimeoutMS int         `json:"timeout_ms"`
	APIKey    SecretInput `json:"api_key"`
}

type UpdateMinifluxConfigInput struct {
	BaseURL       string      `json:"base_url"`
	FetchLimit    int         `json:"fetch_limit"`
	LookbackHours int         `json:"lookback_hours"`
	APIToken      SecretInput `json:"api_token"`
}

type UpdatePublishConfigInput struct {
	Provider    string      `json:"provider"`
	HaloBaseURL string      `json:"halo_base_url"`
	HaloToken   SecretInput `json:"halo_token"`
	OutputDir   string      `json:"output_dir"`
}

type UpdatePromptConfigInput struct {
	TargetLanguage    string `json:"target_language,omitempty"`
	TranslationPrompt string `json:"translation_prompt"`
	AnalysisPrompt    string `json:"analysis_prompt"`
	DossierPrompt     string `json:"dossier_prompt"`
	DigestPrompt      string `json:"digest_prompt"`
}

// GetSnapshot 返回管理员配置快照。
func (s *AdminConfigService) GetSnapshot(ctx context.Context) (AdminConfigSnapshot, error) {
	llmPayload, _, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil {
		return AdminConfigSnapshot{}, err
	}
	minifluxPayload, _, err := s.loadProfile(ctx, profile.TypeMiniflux)
	if err != nil {
		return AdminConfigSnapshot{}, err
	}
	publishPayload, _, err := s.loadProfile(ctx, profile.TypePublish)
	if err != nil {
		return AdminConfigSnapshot{}, err
	}
	promptsPayload, _, err := s.loadProfile(ctx, profile.TypePrompts)
	if err != nil {
		return AdminConfigSnapshot{}, err
	}

	llmAPIKey, err := s.maskedSecretView(stringValue(llmPayload, "api_key"))
	if err != nil {
		return AdminConfigSnapshot{}, err
	}
	minifluxAPIToken, err := s.maskedSecretView(stringValue(minifluxPayload, "api_token"))
	if err != nil {
		return AdminConfigSnapshot{}, err
	}
	publishHaloToken, err := s.maskedSecretView(firstString(publishPayload, "halo_token", "auth_token"))
	if err != nil {
		return AdminConfigSnapshot{}, err
	}

	return AdminConfigSnapshot{
		LLM: LLMConfigView{
			BaseURL:   stringValue(llmPayload, "base_url"),
			Model:     stringValue(llmPayload, "model"),
			TimeoutMS: resolveLLMTimeoutMS(llmPayload, 0),
			APIKey:    llmAPIKey,
		},
		Miniflux: MinifluxConfigView{
			BaseURL:       stringValue(minifluxPayload, "base_url"),
			FetchLimit:    intValue(minifluxPayload, "fetch_limit"),
			LookbackHours: intValue(minifluxPayload, "lookback_hours"),
			APIToken:      minifluxAPIToken,
		},
		Publish: PublishConfigView{
			Provider:    resolvePublishProvider(publishPayload),
			HaloBaseURL: firstString(publishPayload, "halo_base_url", "endpoint"),
			HaloToken:   publishHaloToken,
			OutputDir:   stringValue(publishPayload, "output_dir"),
		},
		Prompts: PromptConfigView{
			TargetLanguage:    stringValue(promptsPayload, "target_language"),
			TranslationPrompt: stringValue(promptsPayload, "translation_prompt"),
			AnalysisPrompt:    stringValue(promptsPayload, "analysis_prompt"),
			DossierPrompt:     stringValue(promptsPayload, "dossier_prompt"),
			DigestPrompt:      stringValue(promptsPayload, "digest_prompt"),
		},
	}, nil
}

// UpdateLLM 更新 LLM 配置。
func (s *AdminConfigService) UpdateLLM(ctx context.Context, input UpdateLLMConfigInput) (profile.Version, error) {
	currentPayload, currentVersion, err := s.loadProfile(ctx, profile.TypeLLM)
	if err != nil {
		return profile.Version{}, err
	}

	payload := clonePayload(currentPayload)
	payload["base_url"] = input.BaseURL
	payload["model"] = input.Model
	payload["timeout_ms"] = resolveLLMTimeoutMS(currentPayload, input.TimeoutMS)

	if err := s.applySecret(payload, currentPayload, "api_key", nil, input.APIKey); err != nil {
		return profile.Version{}, err
	}

	return s.saveProfile(ctx, profile.TypeLLM, "admin-llm", currentVersion, payload)
}

func (s *AdminConfigService) UpdateMiniflux(ctx context.Context, input UpdateMinifluxConfigInput) (profile.Version, error) {
	currentPayload, currentVersion, err := s.loadProfile(ctx, profile.TypeMiniflux)
	if err != nil {
		return profile.Version{}, err
	}

	payload := clonePayload(currentPayload)
	payload["base_url"] = input.BaseURL
	payload["fetch_limit"] = resolvePositiveInt(currentPayload, "fetch_limit", input.FetchLimit)
	payload["lookback_hours"] = resolvePositiveInt(currentPayload, "lookback_hours", input.LookbackHours)

	if err := s.applySecret(payload, currentPayload, "api_token", nil, input.APIToken); err != nil {
		return profile.Version{}, err
	}

	return s.saveProfile(ctx, profile.TypeMiniflux, "admin-miniflux", currentVersion, payload)
}

func (s *AdminConfigService) UpdatePublish(ctx context.Context, input UpdatePublishConfigInput) (profile.Version, error) {
	currentPayload, currentVersion, err := s.loadProfile(ctx, profile.TypePublish)
	if err != nil {
		return profile.Version{}, err
	}

	payload := clonePayload(currentPayload)
	delete(payload, "target_type")
	delete(payload, "endpoint")
	delete(payload, "auth_token")

	provider, err := resolveUpdatedPublishProvider(currentPayload, input.Provider)
	if err != nil {
		return profile.Version{}, err
	}
	payload["provider"] = provider
	payload["halo_base_url"] = input.HaloBaseURL
	payload["output_dir"] = input.OutputDir

	if err := s.applySecret(payload, currentPayload, "halo_token", []string{"auth_token"}, input.HaloToken); err != nil {
		return profile.Version{}, err
	}

	return s.saveProfile(ctx, profile.TypePublish, "admin-publish", currentVersion, payload)
}

func (s *AdminConfigService) UpdatePrompts(ctx context.Context, input UpdatePromptConfigInput) (profile.Version, error) {
	currentPayload, currentVersion, err := s.loadProfile(ctx, profile.TypePrompts)
	if err != nil {
		return profile.Version{}, err
	}

	payload := clonePayload(currentPayload)
	if input.TargetLanguage != "" {
		payload["target_language"] = input.TargetLanguage
	} else if _, ok := payload["target_language"]; !ok {
		payload["target_language"] = ""
	}
	payload["translation_prompt"] = input.TranslationPrompt
	payload["analysis_prompt"] = input.AnalysisPrompt
	payload["dossier_prompt"] = input.DossierPrompt
	payload["digest_prompt"] = input.DigestPrompt

	return s.saveProfile(ctx, profile.TypePrompts, "admin-prompts", currentVersion, payload)
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

func (s *AdminConfigService) saveProfile(ctx context.Context, profileType, name string, currentVersion int, payload map[string]any) (profile.Version, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return profile.Version{}, err
	}

	version := profile.Version{
		ProfileType: profileType,
		Name:        name,
		Version:     nextVersion(currentVersion),
		IsActive:    true,
		PayloadJSON: payloadJSON,
	}

	if err := s.repo.Create(ctx, version); err != nil {
		return profile.Version{}, err
	}

	return version, nil
}

func (s *AdminConfigService) maskedSecretView(value string) (SecretView, error) {
	if value == "" {
		return SecretView{}, nil
	}

	plaintext, err := s.decryptSecret(value)
	if err != nil {
		if security.HasEncryptedPrefix(value) {
			return SecretView{IsSet: true, MaskedValue: "****"}, nil
		}
		return SecretView{}, err
	}
	return maskSecret(plaintext), nil
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

func (s *AdminConfigService) applySecret(payload map[string]any, current map[string]any, key string, legacyKeys []string, input SecretInput) error {
	switch input.Mode {
	case SecretModeReplace:
		if input.Value == "" {
			return errSecretValueRequired
		}
		if s == nil || s.cipher == nil {
			return errSecretCipherRequired
		}
		encrypted, err := s.cipher.EncryptString(input.Value)
		if err != nil {
			return err
		}
		payload[key] = encrypted
	case SecretModeClear:
		payload[key] = ""
	default:
		existing := firstString(current, append([]string{key}, legacyKeys...)...)
		if existing != "" {
			normalized, err := s.normalizeStoredSecret(existing)
			if err != nil {
				return err
			}
			payload[key] = normalized
		}
	}
	return nil
}

func (s *AdminConfigService) normalizeStoredSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if security.HasEncryptedPrefix(value) {
		return value, nil
	}
	if s == nil || s.cipher == nil {
		return value, nil
	}
	return s.cipher.EncryptString(value)
}

func (s *AdminConfigService) decryptSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if s == nil || s.cipher == nil {
		if security.HasEncryptedPrefix(value) {
			return "", errSecretCipherRequired
		}
		return value, nil
	}
	return s.cipher.DecryptString(value)
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

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func resolvePublishProvider(payload map[string]any) string {
	value := firstString(payload, "provider", "target_type")
	if normalized, ok := normalizePublishProviderValue(value); ok {
		return normalized
	}
	return value
}

func resolveUpdatedPublishProvider(payload map[string]any, requested string) (string, error) {
	candidate := requested
	if strings.TrimSpace(candidate) == "" {
		candidate = resolvePublishProvider(payload)
	}
	if normalized, ok := normalizePublishProviderValue(candidate); ok {
		return normalized, nil
	}
	return "", errUnsupportedPublishProvider
}

func normalizePublishProviderValue(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "halo":
		return "halo", true
	case "markdown", "markdown_export":
		return "markdown_export", true
	default:
		return "", false
	}
}

func resolveLLMTimeoutMS(payload map[string]any, value int) int {
	if value > 0 {
		return normalizeAdminLLMTimeoutMS(value)
	}

	current := intValue(payload, "timeout_ms")
	if current > 0 {
		return normalizeAdminLLMTimeoutMS(current)
	}

	return defaultAdminLLMTestTimeoutMS
}

func resolvePositiveInt(payload map[string]any, key string, value int) int {
	if value > 0 {
		return value
	}
	return intValue(payload, key)
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
