package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"rss-platform/internal/config"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/security"
	"rss-platform/internal/service"
)

const adminConfigTestSecretKey = "0123456789abcdef0123456789abcdef"

func newAdminConfigTestCipher(t *testing.T) *security.SecretCipher {
	t.Helper()
	cipher, err := security.NewSecretCipher(adminConfigTestSecretKey)
	if err != nil {
		t.Fatalf("new secret cipher: %v", err)
	}
	return cipher
}

func newAdminConfigService(t *testing.T, repo *profileRepoStub) *service.AdminConfigService {
	t.Helper()
	defaults := &config.Config{}
	defaults.Security.SecretKey = adminConfigTestSecretKey
	return service.NewAdminConfigService(repo, newAdminConfigTestCipher(t), defaults)
}

func decodePayload(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

func encryptedValue(t *testing.T, plaintext string) string {
	t.Helper()
	value, err := newAdminConfigTestCipher(t).EncryptString(plaintext)
	if err != nil {
		t.Fatalf("encrypt %q: %v", plaintext, err)
	}
	return value
}

func decryptValue(t *testing.T, value string) string {
	t.Helper()
	plaintext, err := newAdminConfigTestCipher(t).DecryptString(value)
	if err != nil {
		t.Fatalf("decrypt %q: %v", value, err)
	}
	return plaintext
}

func TestAdminConfigServiceSnapshotMasksEncryptedLLMSecret(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://llm.local/v1","model":"gpt-4.1-mini","api_key":"` + encryptedValue(t, "secret-llm") + `","timeout_ms":45000}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LLM.APIKey.MaskedValue != "secr****" {
		t.Fatalf("want secr**** got %q", snapshot.LLM.APIKey.MaskedValue)
	}
	if snapshot.LLM.TimeoutMS != 45000 {
		t.Fatalf("want timeout_ms=45000 got %d", snapshot.LLM.TimeoutMS)
	}
}

func TestAdminConfigServiceUpdateLLMEncryptsSecretAndKeepsMaskedSnapshot(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://old.local/v1","model":"gpt-4.1-mini","api_key":"` + encryptedValue(t, "secret-llm") + `","timeout_ms":45000,"is_enabled":false}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "gpt-4.1",
		APIKey:  service.SecretInput{Mode: service.SecretModeReplace, Value: "new-secret"},
	}); err != nil {
		t.Fatal(err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("want 1 created got %d", len(repo.created))
	}
	payload := decodePayload(t, repo.created[0].PayloadJSON)
	apiKey, _ := payload["api_key"].(string)
	if !strings.HasPrefix(apiKey, security.EncryptedValuePrefix) {
		t.Fatalf("expected encrypted api_key got %+v", payload)
	}
	if strings.Contains(apiKey, "new-secret") {
		t.Fatalf("plaintext leaked in payload %+v", payload)
	}
	if decryptValue(t, apiKey) != "new-secret" {
		t.Fatalf("want decrypt to new-secret got %+v", payload)
	}
	if payload["timeout_ms"] != float64(45000) {
		t.Fatalf("expected timeout_ms kept got %+v", payload)
	}
	if payload["is_enabled"] != false {
		t.Fatalf("expected is_enabled kept got %+v", payload)
	}

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.LLM.APIKey.IsSet || snapshot.LLM.APIKey.MaskedValue != "new-****" {
		t.Fatalf("unexpected llm secret view %+v", snapshot.LLM.APIKey)
	}
}

func TestAdminConfigServiceUpdateLLMKeepAndClearEncryptedSecret(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"api_key":"` + encryptedValue(t, "secret-llm") + `"}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "gpt-4.1",
		APIKey:  service.SecretInput{Mode: service.SecretModeKeep},
	}); err != nil {
		t.Fatal(err)
	}
	keepPayload := decodePayload(t, repo.created[0].PayloadJSON)
	if decryptValue(t, keepPayload["api_key"].(string)) != "secret-llm" {
		t.Fatalf("want kept encrypted api_key got %+v", keepPayload)
	}

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "gpt-4.1",
		APIKey:  service.SecretInput{Mode: service.SecretModeClear},
	}); err != nil {
		t.Fatal(err)
	}
	clearPayload := decodePayload(t, repo.created[1].PayloadJSON)
	if clearPayload["api_key"] != "" {
		t.Fatalf("want cleared api_key got %+v", clearPayload)
	}
}

func TestAdminConfigServiceUpdateLLMKeepMigratesLegacyPlaintextSecretToEncrypted(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"api_key":"legacy-plain"}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "gpt-4.1",
		APIKey:  service.SecretInput{Mode: service.SecretModeKeep},
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	apiKey, _ := payload["api_key"].(string)
	if !strings.HasPrefix(apiKey, security.EncryptedValuePrefix) {
		t.Fatalf("want plaintext migrated to encrypted got %+v", payload)
	}
	if decryptValue(t, apiKey) != "legacy-plain" {
		t.Fatalf("want migrated secret preserved got %+v", payload)
	}
}

func TestAdminConfigServiceUpdateMinifluxAndSnapshot(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeMiniflux: {
			ProfileType: profile.TypeMiniflux,
			Version:     3,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://miniflux.old","api_token":"` + encryptedValue(t, "old-token") + `","fetch_limit":80,"lookback_hours":12,"is_enabled":true}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	if _, err := svc.UpdateMiniflux(context.Background(), service.UpdateMinifluxConfigInput{
		BaseURL:       "https://miniflux.local",
		FetchLimit:    120,
		LookbackHours: 48,
		APIToken:      service.SecretInput{Mode: service.SecretModeReplace, Value: "fresh-token"},
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if payload["base_url"] != "https://miniflux.local" {
		t.Fatalf("want base_url updated got %+v", payload)
	}
	if payload["fetch_limit"] != float64(120) || payload["lookback_hours"] != float64(48) {
		t.Fatalf("want fetch/lookback updated got %+v", payload)
	}
	if decryptValue(t, payload["api_token"].(string)) != "fresh-token" {
		t.Fatalf("want encrypted api_token got %+v", payload)
	}

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Miniflux.BaseURL != "https://miniflux.local" {
		t.Fatalf("want miniflux base_url got %+v", snapshot.Miniflux)
	}
	if snapshot.Miniflux.FetchLimit != 120 || snapshot.Miniflux.LookbackHours != 48 {
		t.Fatalf("want miniflux ints updated got %+v", snapshot.Miniflux)
	}
	if snapshot.Miniflux.APIToken.MaskedValue != "fres****" {
		t.Fatalf("unexpected miniflux secret view %+v", snapshot.Miniflux.APIToken)
	}
}

func TestAdminConfigServiceUpdateLLMKeepPersistsDefaultSecretFromSeedProfile(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Name:        "default-llm",
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"","model":"","api_key":"","timeout_ms":30000}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Security.SecretKey = adminConfigTestSecretKey
	defaults.LLM.APIKey = "env-llm-secret"
	svc := service.NewAdminConfigService(repo, newAdminConfigTestCipher(t), defaults)

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "MiniMax-M2.7",
		APIKey:  service.SecretInput{Mode: service.SecretModeKeep},
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if decryptValue(t, payload["api_key"].(string)) != "env-llm-secret" {
		t.Fatalf("want env llm secret persisted got %+v", payload)
	}
}

func TestAdminConfigServiceUpdateMinifluxKeepPersistsDefaultSecretFromSeedProfile(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeMiniflux: {
			ProfileType: profile.TypeMiniflux,
			Name:        "default-miniflux",
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"","api_token":"","fetch_limit":100,"lookback_hours":24}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Security.SecretKey = adminConfigTestSecretKey
	defaults.Miniflux.AuthToken = "env-miniflux-secret"
	svc := service.NewAdminConfigService(repo, newAdminConfigTestCipher(t), defaults)

	if _, err := svc.UpdateMiniflux(context.Background(), service.UpdateMinifluxConfigInput{
		BaseURL:       "https://miniflux.local",
		FetchLimit:    100,
		LookbackHours: 48,
		APIToken:      service.SecretInput{Mode: service.SecretModeKeep},
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if decryptValue(t, payload["api_token"].(string)) != "env-miniflux-secret" {
		t.Fatalf("want env miniflux secret persisted got %+v", payload)
	}
}

func TestAdminConfigServiceUpdatePublishKeepPersistsDefaultSecretFromSeedProfile(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Name:        "default-publish",
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_base_url":"","halo_token":"","output_dir":""}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Security.SecretKey = adminConfigTestSecretKey
	defaults.Publish.Channel = "halo"
	defaults.Publish.HaloToken = "env-halo-secret"
	svc := service.NewAdminConfigService(repo, newAdminConfigTestCipher(t), defaults)

	if _, err := svc.UpdatePublish(context.Background(), service.UpdatePublishConfigInput{
		Provider:    "halo",
		HaloBaseURL: "https://halo.local",
		HaloToken:   service.SecretInput{Mode: service.SecretModeKeep},
		OutputDir:   "/tmp/publish",
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if decryptValue(t, payload["halo_token"].(string)) != "env-halo-secret" {
		t.Fatalf("want env halo secret persisted got %+v", payload)
	}
}

func TestAdminConfigServiceSnapshotFallsBackToDefaultsForMinifluxAndMarkdownExport(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Name:        "default-publish",
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_base_url":"","halo_token":"","output_dir":""}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Security.SecretKey = adminConfigTestSecretKey
	defaults.Miniflux.BaseURL = "https://env.miniflux.local"
	defaults.Miniflux.AuthToken = "env-miniflux-token"
	defaults.Publish.OutputDir = "D:/env-output"

	svc := service.NewAdminConfigService(repo, newAdminConfigTestCipher(t), defaults)
	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Miniflux.BaseURL != "https://env.miniflux.local" {
		t.Fatalf("want env miniflux base_url got %q", snapshot.Miniflux.BaseURL)
	}
	if !snapshot.Miniflux.APIToken.IsSet {
		t.Fatalf("want miniflux token configured got %+v", snapshot.Miniflux.APIToken)
	}
	if snapshot.Publish.Provider != "markdown_export" {
		t.Fatalf("want markdown_export provider got %q", snapshot.Publish.Provider)
	}
	if snapshot.Publish.OutputDir != "D:/env-output" {
		t.Fatalf("want env output_dir got %q", snapshot.Publish.OutputDir)
	}
	if snapshot.Publish.HaloBaseURL != "" || snapshot.Publish.HaloToken.IsSet {
		t.Fatalf("default publish seed should not force halo config %+v", snapshot.Publish)
	}
}

func TestAdminConfigServiceUpdatePublishNormalizesLegacyPayloadAndSnapshot(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Version:     4,
			IsActive:    true,
			PayloadJSON: []byte(`{"target_type":"halo","endpoint":"https://halo.legacy","auth_token":"` + encryptedValue(t, "legacy-token") + `","content_format":"markdown","is_enabled":true}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Publish.Provider != "halo" || snapshot.Publish.HaloBaseURL != "https://halo.legacy" {
		t.Fatalf("unexpected legacy publish snapshot %+v", snapshot.Publish)
	}
	if snapshot.Publish.HaloToken.MaskedValue != "lega****" {
		t.Fatalf("unexpected publish secret view %+v", snapshot.Publish.HaloToken)
	}

	if _, err := svc.UpdatePublish(context.Background(), service.UpdatePublishConfigInput{
		Provider:    "markdown_export",
		HaloBaseURL: "https://halo.local",
		HaloToken:   service.SecretInput{Mode: service.SecretModeReplace, Value: "halo-secret"},
		OutputDir:   "data/output",
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if payload["provider"] != "markdown_export" {
		t.Fatalf("want provider normalized got %+v", payload)
	}
	if payload["halo_base_url"] != "https://halo.local" || payload["output_dir"] != "data/output" {
		t.Fatalf("want publish fields updated got %+v", payload)
	}
	if decryptValue(t, payload["halo_token"].(string)) != "halo-secret" {
		t.Fatalf("want encrypted halo_token got %+v", payload)
	}
	if _, ok := payload["target_type"]; ok {
		t.Fatalf("did not expect legacy target_type in payload %+v", payload)
	}
	if _, ok := payload["endpoint"]; ok {
		t.Fatalf("did not expect legacy endpoint in payload %+v", payload)
	}
	if _, ok := payload["auth_token"]; ok {
		t.Fatalf("did not expect legacy auth_token in payload %+v", payload)
	}
}

func TestAdminConfigServiceSnapshotKeepsConfiguredStateWhenCipherMissing(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"api_key":"` + encryptedValue(t, "secret-llm") + `"}`),
		},
	}}
	svc := service.NewAdminConfigService(repo, nil, nil)

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("want snapshot degrade instead of error, got %v", err)
	}
	if !snapshot.LLM.APIKey.IsSet {
		t.Fatalf("want secret marked configured got %+v", snapshot.LLM.APIKey)
	}
	if snapshot.LLM.APIKey.MaskedValue == "" {
		t.Fatalf("want generic mask for encrypted secret got %+v", snapshot.LLM.APIKey)
	}
}

func TestAdminConfigServiceSnapshotKeepsConfiguredStateWhenEncryptedSecretInvalid(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_token":"` + security.EncryptedValuePrefix + `not-valid-base64"}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("want snapshot degrade instead of error, got %v", err)
	}
	if !snapshot.Publish.HaloToken.IsSet {
		t.Fatalf("want secret marked configured got %+v", snapshot.Publish.HaloToken)
	}
	if snapshot.Publish.HaloToken.MaskedValue == "" {
		t.Fatalf("want generic mask for broken encrypted secret got %+v", snapshot.Publish.HaloToken)
	}
}

func TestAdminConfigServiceUpdatePublishNormalizesMarkdownAlias(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Version:     4,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_token":"` + encryptedValue(t, "legacy-token") + `"}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	if _, err := svc.UpdatePublish(context.Background(), service.UpdatePublishConfigInput{
		Provider:  "markdown",
		HaloToken: service.SecretInput{Mode: service.SecretModeKeep},
		OutputDir: "data/output",
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if payload["provider"] != "markdown_export" {
		t.Fatalf("want markdown alias normalized got %+v", payload)
	}
}

func TestAdminConfigServiceUpdatePublishRejectsInvalidProvider(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Version:     4,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_token":"` + encryptedValue(t, "legacy-token") + `"}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	_, err := svc.UpdatePublish(context.Background(), service.UpdatePublishConfigInput{
		Provider:  "invalid",
		HaloToken: service.SecretInput{Mode: service.SecretModeKeep},
		OutputDir: "data/output",
	})
	if err == nil {
		t.Fatal("want error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported publish provider") {
		t.Fatalf("want whitelist error got %v", err)
	}
}

func TestAdminConfigServiceUpdatePromptsAndSnapshot(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePrompts: {
			ProfileType: profile.TypePrompts,
			Version:     5,
			IsActive:    true,
			PayloadJSON: []byte(`{"target_language":"zh-CN","translation_prompt":"old-T","analysis_prompt":"old-A","dossier_prompt":"old-D","digest_prompt":"old-G","is_enabled":true}`),
		},
	}}
	svc := newAdminConfigService(t, repo)

	if _, err := svc.UpdatePrompts(context.Background(), service.UpdatePromptConfigInput{
		TranslationPrompt: "new-T",
		AnalysisPrompt:    "new-A",
		DossierPrompt:     "new-D",
		DigestPrompt:      "new-G",
	}); err != nil {
		t.Fatal(err)
	}

	payload := decodePayload(t, repo.created[0].PayloadJSON)
	if payload["translation_prompt"] != "new-T" || payload["analysis_prompt"] != "new-A" || payload["dossier_prompt"] != "new-D" || payload["digest_prompt"] != "new-G" {
		t.Fatalf("unexpected prompts payload %+v", payload)
	}
	if payload["target_language"] != "zh-CN" {
		t.Fatalf("want target_language preserved got %+v", payload)
	}

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Prompts.TranslationPrompt != "new-T" || snapshot.Prompts.AnalysisPrompt != "new-A" || snapshot.Prompts.DossierPrompt != "new-D" || snapshot.Prompts.DigestPrompt != "new-G" {
		t.Fatalf("unexpected prompts snapshot %+v", snapshot.Prompts)
	}
	if snapshot.Prompts.TargetLanguage != "zh-CN" {
		t.Fatalf("want target_language preserved got %+v", snapshot.Prompts)
	}
}
