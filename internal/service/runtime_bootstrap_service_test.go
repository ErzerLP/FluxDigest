package service_test

import (
	"context"
	"errors"
	"testing"

	"rss-platform/internal/service"
)

type bootstrapState struct {
	lockHeld           *bool
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

	if s.state.lockHeld == nil {
		s.state.lockHeld = new(bool)
	}
	*s.state.lockHeld = true
	defer func() {
		*s.state.lockHeld = false
	}()

	return run(ctx)
}

func (s *bootstrapMigratorStub) Migrate(_ context.Context) error {
	s.state.migrateCalls++
	if s.state.lockHeld == nil || !*s.state.lockHeld {
		s.state.migrateOutsideLock = true
	}
	return s.state.migrateErr
}

type bootstrapSeederStub struct {
	state *bootstrapState
}

func (s *bootstrapSeederStub) SeedDefaults(_ context.Context) error {
	s.state.seedCalls++
	if s.state.lockHeld == nil || !*s.state.lockHeld {
		s.state.seedOutsideLock = true
	}
	return s.state.seedErr
}

func TestBootstrapServiceRunsMigratorAndSeedsDefaultsWithinLock(t *testing.T) {
	lockHeld := false
	state := &bootstrapState{lockHeld: &lockHeld}
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

func TestBootstrapServiceRunsAllSeeders(t *testing.T) {
	lockHeld := false
	stateA := &bootstrapState{lockHeld: &lockHeld}
	stateB := &bootstrapState{lockHeld: &lockHeld}
	svc := service.NewRuntimeBootstrapService(
		&bootstrapMigratorStub{state: stateA},
		&bootstrapSeederStub{state: stateA},
		&bootstrapSeederStub{state: stateB},
	)

	if err := svc.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if stateA.seedCalls != 1 {
		t.Fatalf("want first seeder called once got %d", stateA.seedCalls)
	}
	if stateB.seedCalls != 1 {
		t.Fatalf("want second seeder called once got %d", stateB.seedCalls)
	}
	if stateA.seedOutsideLock {
		t.Fatal("first seeder ran outside lock")
	}
	if stateB.seedOutsideLock {
		t.Fatal("second seeder ran outside lock")
	}
}

func TestBootstrapServiceDoesNotSeedWhenMigrateFails(t *testing.T) {
	lockHeld := false
	errMigrateFailed := errors.New("migrate failed")
	state := &bootstrapState{lockHeld: &lockHeld, migrateErr: errMigrateFailed}
	svc := service.NewRuntimeBootstrapService(
		&bootstrapMigratorStub{state: state},
		&bootstrapSeederStub{state: state},
	)

	err := svc.Ensure(context.Background())
	if !errors.Is(err, errMigrateFailed) {
		t.Fatalf("want migrate error wrapped got %v", err)
	}
	if state.seedCalls != 0 {
		t.Fatalf("want no seed call got %d", state.seedCalls)
	}
}

func TestBootstrapServiceReturnsSeedError(t *testing.T) {
	lockHeld := false
	errSeedFailed := errors.New("seed failed")
	state := &bootstrapState{lockHeld: &lockHeld, seedErr: errSeedFailed}
	svc := service.NewRuntimeBootstrapService(
		&bootstrapMigratorStub{state: state},
		&bootstrapSeederStub{state: state},
	)

	err := svc.Ensure(context.Background())
	if !errors.Is(err, errSeedFailed) {
		t.Fatalf("want seed error wrapped got %v", err)
	}
	if state.migrateCalls != 1 {
		t.Fatalf("want 1 migrate call got %d", state.migrateCalls)
	}
	if state.seedCalls != 1 {
		t.Fatalf("want 1 seed call got %d", state.seedCalls)
	}
}
