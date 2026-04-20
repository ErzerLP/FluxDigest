package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"rss-platform/internal/config"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/security"
)

// LLMRuntimeConfig 表示 worker 使用的 LLM 运行时配置。
type LLMRuntimeConfig struct {
	BaseURL        string   `json:"base_url"`
	APIKey         string   `json:"api_key"`
	Model          string   `json:"model"`
	FallbackModels []string `json:"fallback_models"`
	TimeoutMS      int      `json:"timeout_ms"`
	Version        int      `json:"version"`
}

// MinifluxRuntimeConfig 表示 worker 使用的 Miniflux 运行时配置。
type MinifluxRuntimeConfig struct {
	BaseURL       string `json:"base_url"`
	AuthToken     string `json:"auth_token"`
	FetchLimit    int    `json:"fetch_limit"`
	LookbackHours int    `json:"lookback_hours"`
	Version       int    `json:"version"`
}

// PublishRuntimeConfig 表示 worker 使用的发布运行时配置。
type PublishRuntimeConfig struct {
	Provider           string `json:"provider"`
	HaloBaseURL        string `json:"halo_base_url"`
	HaloToken          string `json:"halo_token"`
	OutputDir          string `json:"output_dir"`
	ArticlePublishMode string `json:"article_publish_mode"`
	ArticleReviewMode  string `json:"article_review_mode"`
	Version            int    `json:"version"`
}

// PromptRuntimeConfig 表示 worker 使用的 prompt 运行时配置。
type PromptRuntimeConfig struct {
	TranslationPrompt  string `json:"translation_prompt"`
	AnalysisPrompt     string `json:"analysis_prompt"`
	DossierPrompt      string `json:"dossier_prompt"`
	DigestPrompt       string `json:"digest_prompt"`
	TranslationVersion int    `json:"translation_version"`
	AnalysisVersion    int    `json:"analysis_version"`
	DossierVersion     int    `json:"dossier_version"`
	DigestVersion      int    `json:"digest_version"`
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
	Miniflux  MinifluxRuntimeConfig  `json:"miniflux"`
	Publish   PublishRuntimeConfig   `json:"publish"`
	Prompts   PromptRuntimeConfig    `json:"prompts"`
	Scheduler SchedulerRuntimeConfig `json:"scheduler"`
}

// RuntimeConfigService 负责按 DB 优先、配置兜底读取运行时配置。
type RuntimeConfigService struct {
	repo      ProfileRepository
	defaults  *config.Config
	cipher    *security.SecretCipher
	cipherErr error
}

type activeProfileSnapshot struct {
	version profile.Version
	payload map[string]any
}

const defaultMinifluxFetchLimit = 100
const defaultMinifluxLookbackHours = 24

// NewRuntimeConfigService 创建 RuntimeConfigService。
func NewRuntimeConfigService(repo ProfileRepository, defaults *config.Config) *RuntimeConfigService {
	svc := &RuntimeConfigService{repo: repo, defaults: defaults}
	if defaults != nil && strings.TrimSpace(defaults.Security.SecretKey) != "" {
		svc.cipher, svc.cipherErr = security.NewSecretCipher(strings.TrimSpace(defaults.Security.SecretKey))
	}
	return svc
}

// Snapshot 返回运行时配置快照。
func (s *RuntimeConfigService) Snapshot(ctx context.Context) (RuntimeSnapshot, error) {
	snapshot := RuntimeSnapshot{
		LLM: LLMRuntimeConfig{
			BaseURL:        s.defaultLLMBaseURL(),
			APIKey:         s.defaultLLMAPIKey(),
			Model:          s.defaultLLMModel(),
			FallbackModels: s.defaultLLMFallbackModels(),
			TimeoutMS:      s.defaultLLMTimeoutMS(),
		},
		Miniflux: MinifluxRuntimeConfig{
			BaseURL:       s.defaultMinifluxBaseURL(),
			AuthToken:     s.defaultMinifluxAuthToken(),
			FetchLimit:    defaultMinifluxFetchLimit,
			LookbackHours: defaultMinifluxLookbackHours,
		},
		Publish: PublishRuntimeConfig{
			Provider:           ResolvePublishProvider(s.defaultPublishChannel(), s.defaultPublishHaloBaseURL(), s.defaultPublishOutputDir()),
			HaloBaseURL:        s.defaultPublishHaloBaseURL(),
			HaloToken:          s.defaultPublishHaloToken(),
			OutputDir:          s.defaultPublishOutputDir(),
			ArticlePublishMode: normalizeArticlePublishMode(""),
			ArticleReviewMode:  normalizeArticleReviewMode(""),
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
		value, err := s.resolveSecretString(stringValue(llmProfile.payload, "api_key"))
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		snapshot.LLM.APIKey = value
	} else if value := strings.TrimSpace(stringValue(llmProfile.payload, "api_key")); value != "" {
		resolved, err := s.resolveSecretString(value)
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		snapshot.LLM.APIKey = resolved
	}
	if s.shouldUseExplicitStringOverride(llmProfile.version, "model", llmProfile.payload) {
		snapshot.LLM.Model = stringValue(llmProfile.payload, "model")
	} else if value := strings.TrimSpace(stringValue(llmProfile.payload, "model")); value != "" {
		snapshot.LLM.Model = value
	}
	if value := intValue(llmProfile.payload, "timeout_ms"); value > 0 {
		snapshot.LLM.TimeoutMS = value
	}
	if values := stringSliceValue(llmProfile.payload, "fallback_models"); len(values) > 0 {
		snapshot.LLM.FallbackModels = values
	}
	snapshot.LLM.Version = llmProfile.version.Version

	minifluxProfile, err := s.activeProfile(ctx, profile.TypeMiniflux)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if s.shouldUseExplicitStringOverride(minifluxProfile.version, "base_url", minifluxProfile.payload) {
		snapshot.Miniflux.BaseURL = stringValue(minifluxProfile.payload, "base_url")
	} else if value := strings.TrimSpace(stringValue(minifluxProfile.payload, "base_url")); value != "" {
		snapshot.Miniflux.BaseURL = value
	}
	if s.shouldUseExplicitStringOverride(minifluxProfile.version, "api_token", minifluxProfile.payload) {
		value, err := s.resolveSecretString(stringValue(minifluxProfile.payload, "api_token"))
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		snapshot.Miniflux.AuthToken = value
	} else if value := strings.TrimSpace(stringValue(minifluxProfile.payload, "api_token")); value != "" {
		resolved, err := s.resolveSecretString(value)
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		snapshot.Miniflux.AuthToken = resolved
	}
	if value := intValue(minifluxProfile.payload, "fetch_limit"); value > 0 {
		snapshot.Miniflux.FetchLimit = value
	}
	if value := intValue(minifluxProfile.payload, "lookback_hours"); value > 0 {
		snapshot.Miniflux.LookbackHours = value
	}
	snapshot.Miniflux.Version = minifluxProfile.version.Version

	publishProfile, err := s.activeProfile(ctx, profile.TypePublish)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	if s.shouldUseExplicitStringOverride(publishProfile.version, "provider", publishProfile.payload) {
		snapshot.Publish.Provider = stringValue(publishProfile.payload, "provider")
	} else if !isDefaultSeedProfile(publishProfile.version) {
		if value := strings.TrimSpace(firstString(publishProfile.payload, "provider", "target_type")); value != "" {
			snapshot.Publish.Provider = value
		}
	}
	if s.shouldUseExplicitStringOverride(publishProfile.version, "halo_base_url", publishProfile.payload) {
		snapshot.Publish.HaloBaseURL = stringValue(publishProfile.payload, "halo_base_url")
	} else if value := strings.TrimSpace(firstString(publishProfile.payload, "halo_base_url", "endpoint")); value != "" {
		snapshot.Publish.HaloBaseURL = value
	}
	if s.shouldUseExplicitStringOverride(publishProfile.version, "halo_token", publishProfile.payload) {
		value, err := s.resolveSecretString(firstString(publishProfile.payload, "halo_token", "auth_token"))
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		snapshot.Publish.HaloToken = value
	} else if value := strings.TrimSpace(firstString(publishProfile.payload, "halo_token", "auth_token")); value != "" {
		resolved, err := s.resolveSecretString(value)
		if err != nil {
			return RuntimeSnapshot{}, err
		}
		snapshot.Publish.HaloToken = resolved
	}
	if s.shouldUseExplicitStringOverride(publishProfile.version, "output_dir", publishProfile.payload) {
		snapshot.Publish.OutputDir = stringValue(publishProfile.payload, "output_dir")
	} else if value := strings.TrimSpace(stringValue(publishProfile.payload, "output_dir")); value != "" {
		snapshot.Publish.OutputDir = value
	}
	snapshot.Publish.ArticlePublishMode = normalizeArticlePublishMode(stringValue(publishProfile.payload, "article_publish_mode"))
	snapshot.Publish.ArticleReviewMode = normalizeArticleReviewMode(stringValue(publishProfile.payload, "article_review_mode"))
	snapshot.Publish.Provider = ResolvePublishProvider(snapshot.Publish.Provider, snapshot.Publish.HaloBaseURL, snapshot.Publish.OutputDir)
	snapshot.Publish.Version = publishProfile.version.Version

	promptsProfile, err := s.activeProfile(ctx, profile.TypePrompts)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	snapshot.Prompts.TranslationPrompt = stringValue(promptsProfile.payload, "translation_prompt")
	snapshot.Prompts.AnalysisPrompt = stringValue(promptsProfile.payload, "analysis_prompt")
	snapshot.Prompts.DossierPrompt = stringValue(promptsProfile.payload, "dossier_prompt")
	snapshot.Prompts.DigestPrompt = stringValue(promptsProfile.payload, "digest_prompt")
	snapshot.Prompts.TranslationVersion = promptsProfile.version.Version
	snapshot.Prompts.AnalysisVersion = promptsProfile.version.Version
	snapshot.Prompts.DossierVersion = promptsProfile.version.Version
	snapshot.Prompts.DigestVersion = promptsProfile.version.Version

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

// Miniflux 返回 Miniflux 运行时配置。
func (s *RuntimeConfigService) Miniflux(ctx context.Context) (MinifluxRuntimeConfig, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return MinifluxRuntimeConfig{}, err
	}
	return snapshot.Miniflux, nil
}

// Publish 返回发布运行时配置。
func (s *RuntimeConfigService) Publish(ctx context.Context) (PublishRuntimeConfig, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return PublishRuntimeConfig{}, err
	}
	return snapshot.Publish, nil
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
		return "MiniMax-M2.7"
	}
	if strings.TrimSpace(s.defaults.LLM.Model) != "" {
		return s.defaults.LLM.Model
	}
	return "MiniMax-M2.7"
}

func (s *RuntimeConfigService) defaultLLMFallbackModels() []string {
	if s == nil || s.defaults == nil || len(s.defaults.LLM.FallbackModels) == 0 {
		return []string{"mimo-v2-pro"}
	}
	return append([]string(nil), s.defaults.LLM.FallbackModels...)
}

func (s *RuntimeConfigService) defaultLLMTimeoutMS() int {
	if s == nil || s.defaults == nil {
		return 30000
	}
	if s.defaults.LLM.TimeoutMS > 0 {
		return s.defaults.LLM.TimeoutMS
	}
	return 30000
}

func (s *RuntimeConfigService) defaultMinifluxBaseURL() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return strings.TrimSpace(s.defaults.Miniflux.BaseURL)
}

func (s *RuntimeConfigService) defaultMinifluxAuthToken() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return strings.TrimSpace(s.defaults.Miniflux.AuthToken)
}

func (s *RuntimeConfigService) defaultPublishChannel() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return strings.TrimSpace(s.defaults.Publish.Channel)
}

func (s *RuntimeConfigService) defaultPublishHaloBaseURL() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return strings.TrimSpace(s.defaults.Publish.HaloBaseURL)
}

func (s *RuntimeConfigService) defaultPublishHaloToken() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return strings.TrimSpace(s.defaults.Publish.HaloToken)
}

func (s *RuntimeConfigService) defaultPublishOutputDir() string {
	if s == nil || s.defaults == nil {
		return ""
	}
	return strings.TrimSpace(s.defaults.Publish.OutputDir)
}

// ResolvePublishProvider 解析实际生效的发布器类型。
func ResolvePublishProvider(provider, haloBaseURL, outputDir string) string {
	if normalized, ok := normalizePublishProviderValue(provider); ok {
		return normalized
	}

	if strings.TrimSpace(provider) == "" {
		if strings.TrimSpace(haloBaseURL) != "" {
			return "halo"
		}
		return "markdown_export"
	}

	return strings.ToLower(strings.TrimSpace(provider))
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

func stringSliceValue(payload map[string]any, key string) []string {
	if payload == nil {
		return nil
	}

	value, ok := payload[key]
	if !ok {
		return nil
	}

	switch cast := value.(type) {
	case []string:
		return filterNonEmptyStrings(cast)
	case []any:
		items := make([]string, 0, len(cast))
		for _, item := range cast {
			text, ok := item.(string)
			if !ok {
				continue
			}
			items = append(items, text)
		}
		return filterNonEmptyStrings(items)
	case string:
		return filterNonEmptyStrings(strings.Split(cast, ","))
	default:
		return nil
	}
}

func filterNonEmptyStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
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

func (s *RuntimeConfigService) resolveSecretString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !security.HasEncryptedPrefix(value) {
		return value, nil
	}
	if s != nil && s.cipherErr != nil {
		return "", s.cipherErr
	}
	if s == nil || s.cipher == nil {
		return "", errors.New("runtime secret cipher is required")
	}
	return s.cipher.DecryptString(value)
}
