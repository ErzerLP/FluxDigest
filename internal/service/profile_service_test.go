package service_test

import (
	"context"
	"encoding/json"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
	"testing"
)

type profileRepoStub struct {
	created []profile.Version
	active  map[string]profile.Version
}

func (s *profileRepoStub) Create(_ context.Context, v profile.Version) error {
	s.created = append(s.created, v)
	if s.active == nil {
		s.active = make(map[string]profile.Version)
	}
	if v.IsActive {
		s.active[v.ProfileType] = v
	}
	return nil
}

func (s *profileRepoStub) GetActive(_ context.Context, profileType string) (profile.Version, error) {
	if s.active != nil {
		if v, ok := s.active[profileType]; ok {
			return v, nil
		}
	}
	return profile.Version{}, profile.ErrNotFound
}

func TestProfileServiceSeedsDefaults(t *testing.T) {
	repo := &profileRepoStub{}
	svc := service.NewProfileService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 5 {
		t.Fatalf("want 5 got %d", len(repo.created))
	}

	var llmPayload map[string]any
	if err := json.Unmarshal(repo.created[0].PayloadJSON, &llmPayload); err != nil {
		t.Fatalf("unmarshal llm payload: %v", err)
	}
	if llmPayload["model"] != "MiniMax-M2.7" {
		t.Fatalf("missing default llm model in payload: %+v", llmPayload)
	}
	fallbackModels, ok := llmPayload["fallback_models"].([]any)
	if !ok || len(fallbackModels) != 1 || fallbackModels[0] != "mimo-v2-pro" {
		t.Fatalf("missing default fallback models in payload: %+v", llmPayload)
	}
	if llmPayload["timeout_ms"] != float64(30000) {
		t.Fatalf("missing default llm timeout in payload: %+v", llmPayload)
	}

	var publishPayload map[string]any
	if err := json.Unmarshal(repo.created[3].PayloadJSON, &publishPayload); err != nil {
		t.Fatalf("unmarshal publish payload: %v", err)
	}
	if publishPayload["target_type"] != "halo" {
		t.Fatalf("want default publish target_type halo got %+v", publishPayload["target_type"])
	}
}

func TestProfileServiceSeedDefaultsIsIdempotentWhenActiveExists(t *testing.T) {
	repo := &profileRepoStub{}
	svc := service.NewProfileService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	if len(repo.created) != 5 {
		t.Fatalf("want 5 created records after two seed calls, got %d", len(repo.created))
	}
}

func TestProfileServiceSeedsPromptDefaultsIncludingDossierPrompt(t *testing.T) {
	repo := &profileRepoStub{}
	svc := service.NewProfileService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}

	var promptsPayload map[string]any
	if err := json.Unmarshal(repo.created[2].PayloadJSON, &promptsPayload); err != nil {
		t.Fatalf("unmarshal prompts payload: %v", err)
	}
	if promptsPayload["translation_prompt"] == "" {
		t.Fatalf("translation prompt should not be empty: %+v", promptsPayload)
	}
	if promptsPayload["analysis_prompt"] == "" {
		t.Fatalf("analysis prompt should not be empty: %+v", promptsPayload)
	}
	if promptsPayload["dossier_prompt"] == "" {
		t.Fatalf("dossier prompt should not be empty: %+v", promptsPayload)
	}
	if promptsPayload["digest_prompt"] == "" {
		t.Fatalf("digest prompt should not be empty: %+v", promptsPayload)
	}
}
