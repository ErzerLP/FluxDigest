package service_test

import (
	"context"
	"testing"

	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
)

type profileRepoStub struct {
	created []profile.Version
}

func (s *profileRepoStub) Create(_ context.Context, v profile.Version) error {
	s.created = append(s.created, v)
	return nil
}

func TestProfileServiceSeedsDefaults(t *testing.T) {
	repo := &profileRepoStub{}
	svc := service.NewProfileService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 4 {
		t.Fatalf("want 4 got %d", len(repo.created))
	}
}
