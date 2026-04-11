package service_test

import (
	"context"
	"errors"
	"testing"

	"rss-platform/internal/service"
)

type bootstrapState struct {
	locked             bool
	lockCalls          int
	migrateCalls       int
	seedCalls          int
	migrateOutsideLock bool
	seedOutsideLock    bool
	lockErr            error
	migrateErr         error
	seedErr            error
}

type bootstrapMigratorStub struct {
	state *bootstrapState
}

func (s *bootstrapMigratorStub) WithLock(ctx context.Context, run func(context.Context) error) error {
	s.state.lockCalls++
	if s.state.lockErr != nil {
		return s.state.lockErr
	}

	s.state.locked = true
	defer func() {
		s.state.locked = false
	}()

	return run(ctx)
}

func (s *bootstrapMigratorStub) Migrate(_ context.Context) error {
	s.state.migrateCalls++
	if !s.state.locked {
		s.state.migrateOutsideLock = true
	}
	return s.state.migrateErr
}

type bootstrapSeederStub struct {
	state *bootstrapState
}

func (s *bootstrapSeederStub) SeedDefaults(_ context.Context) error {
	s.state.seedCalls++
	if !s.state.locked {
		s.state.seedOutsideLock = true
	}
	return s.state.seedErr
}

func TestBootstrapServiceRunsMigratorAndSeedsDefaultsWithinLock(t *testing.T) {
	state := &bootstrapState{}
	svc := service.NewRuntimeBootstrapService(
		&bootstrapMigratorStub{state: state},
		&bootstrapSeederStub{state: state},
	)

	if err := svc.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if state.lockCalls != 1 {
		t.Fatalf("want 1 lock call got %d", state.lockCalls)
	}
	if state.migrateCalls != 1 {
		t.Fatalf("want 1 migrate call got %d", state.migrateCalls)
	}
	if state.seedCalls != 1 {
		t.Fatalf("want 1 seed call got %d", state.seedCalls)
	}
	if state.migrateOutsideLock {
		t.Fatal("migrate ran outside lock")
	}
	if state.seedOutsideLock {
		t.Fatal("seed ran outside lock")
	}
}

func TestBootstrapServiceDoesNotSeedWhenMigrateFails(t *testing.T) {
	state := &bootstrapState{migrateErr: errors.New("migrate failed")}
	svc := service.NewRuntimeBootstrapService(
		&bootstrapMigratorStub{state: state},
		&bootstrapSeederStub{state: state},
	)

	err := svc.Ensure(context.Background())
	if err == nil || err.Error() != "migrate failed" {
		t.Fatalf("want migrate failed got %v", err)
	}
	if state.seedCalls != 0 {
		t.Fatalf("want no seed call got %d", state.seedCalls)
	}
}

func TestBootstrapServiceReturnsSeedError(t *testing.T) {
	state := &bootstrapState{seedErr: errors.New("seed failed")}
	svc := service.NewRuntimeBootstrapService(
		&bootstrapMigratorStub{state: state},
		&bootstrapSeederStub{state: state},
	)

	err := svc.Ensure(context.Background())
	if err == nil || err.Error() != "seed failed" {
		t.Fatalf("want seed failed got %v", err)
	}
	if state.migrateCalls != 1 {
		t.Fatalf("want 1 migrate call got %d", state.migrateCalls)
	}
	if state.seedCalls != 1 {
		t.Fatalf("want 1 seed call got %d", state.seedCalls)
	}
}
