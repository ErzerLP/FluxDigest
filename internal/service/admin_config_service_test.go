package service_test

import (
	"context"
	"encoding/json"
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
			PayloadJSON: []byte(`{"base_url":"https://llm.local/v1","model":"gpt-4.1-mini","api_key":"secret-llm","is_enabled":true,"timeout_ms":45000}`),
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
	if snapshot.LLM.TimeoutMS != 45000 {
		t.Fatalf("want timeout_ms=45000 got %d", snapshot.LLM.TimeoutMS)
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

func TestAdminConfigServiceUpdateLLMMergeAndVersion(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://old.local/v1","model":"gpt-4.1-mini","api_key":"secret-llm","timeout_ms":45000,"is_enabled":false}`),
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

	if len(repo.created) != 1 {
		t.Fatalf("want 1 created got %d", len(repo.created))
	}
	if repo.created[0].Version != 3 {
		t.Fatalf("want version 3 got %d", repo.created[0].Version)
	}

	var payload map[string]any
	if err := json.Unmarshal(repo.created[0].PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["timeout_ms"] != float64(45000) {
		t.Fatalf("expected timeout_ms kept got %+v", payload)
	}
	if payload["is_enabled"] != false {
		t.Fatalf("expected is_enabled kept got %+v", payload)
	}
	if payload["base_url"] != "https://proxy.local/v1" || payload["model"] != "gpt-4.1" {
		t.Fatalf("expected updated base_url/model got %+v", payload)
	}
	if payload["api_key"] != "secret-llm" {
		t.Fatalf("expected api_key kept got %+v", payload)
	}
}

func TestAdminConfigServiceUpdateLLMReplaceRequiresValue(t *testing.T) {
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
		APIKey:  service.SecretInput{Mode: service.SecretModeReplace},
	}); err == nil {
		t.Fatal("expected error on replace without value")
	}
	if len(repo.created) != 0 {
		t.Fatalf("expected no create on error got %d", len(repo.created))
	}
}

func TestAdminConfigServiceUpdateLLMCanSetTimeoutMS(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://old.local/v1","model":"gpt-4.1-mini","api_key":"secret-llm","timeout_ms":30000}`),
		},
	}}
	svc := service.NewAdminConfigService(repo)

	timeoutMS := 60000
	if _, err := svc.UpdateLLM(context.Background(), service.UpdateLLMConfigInput{
		BaseURL:   "https://proxy.local/v1",
		Model:     "gpt-4.1",
		TimeoutMS: timeoutMS,
		APIKey:    service.SecretInput{Mode: service.SecretModeKeep},
	}); err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(repo.created[0].PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["timeout_ms"] != float64(60000) {
		t.Fatalf("expected timeout_ms=60000 got %+v", payload)
	}
}
