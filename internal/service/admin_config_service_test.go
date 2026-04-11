package service_test

import (
	"context"
	"strings"
	"testing"

	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
)

func TestAdminConfigServiceSnapshotMasksSecrets(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://llm.local/v1","model":"gpt-4.1-mini","api_key":"secret-llm","is_enabled":true}`),
		},
	}}
	svc := service.NewAdminConfigService(repo)

	snapshot, err := svc.GetSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LLM.APIKey.MaskedValue != "secr****" {
		t.Fatalf("want secr**** got %q", snapshot.LLM.APIKey.MaskedValue)
	}
}

func TestAdminConfigServiceUpdateLLMKeepAndClearSecret(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"api_key":"secret-llm"}`),
		},
	}}
	svc := service.NewAdminConfigService(repo)

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "gpt-4.1",
		APIKey:  service.SecretInput{Mode: service.SecretModeKeep},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repo.created[0].PayloadJSON), `"api_key":"secret-llm"`) {
		t.Fatalf("expected kept secret payload got %s", repo.created[0].PayloadJSON)
	}

	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL: "https://proxy.local/v1",
		Model:   "gpt-4.1",
		APIKey:  service.SecretInput{Mode: service.SecretModeClear},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(repo.created[1].PayloadJSON), `"api_key":"secret-llm"`) {
		t.Fatalf("expected cleared secret payload got %s", repo.created[1].PayloadJSON)
	}
}
