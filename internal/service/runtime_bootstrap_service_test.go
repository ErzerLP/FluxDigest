package service_test

import (
	"context"
	"testing"

	"rss-platform/internal/service"
)

type migratorStub struct {
	calls int
	err   error
}

func (s *migratorStub) Migrate(_ context.Context) error {
	s.calls++
	return s.err
}

func TestBootstrapServiceRunsMigratorAndSeedsDefaults(t *testing.T) {
	migrator := &migratorStub{}
	profiles := &profileRepoStub{}
	svc := service.NewRuntimeBootstrapService(migrator, service.NewProfileService(profiles))

	if err := svc.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if migrator.calls != 1 {
		t.Fatalf("want 1 migrate call got %d", migrator.calls)
	}
	if len(profiles.created) != 4 {
		t.Fatalf("want 4 seeded profiles got %d", len(profiles.created))
	}
}
