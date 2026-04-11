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

	var aiPayload map[string]any
	if err := json.Unmarshal(repo.created[0].PayloadJSON, &aiPayload); err != nil {
		t.Fatalf("unmarshal llm payload: %v", err)
	}
	if aiPayload["model"] != "gpt-4.1-mini" {
		t.Fatalf("missing default llm model in payload: %+v", aiPayload)
	}
	if aiPayload["timeout_ms"] != float64(30000) {
		t.Fatalf("missing default llm timeout in payload: %+v", aiPayload)
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
