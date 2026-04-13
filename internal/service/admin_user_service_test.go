package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"rss-platform/internal/domain/admin"
	"rss-platform/internal/service"
)

type adminUserRepoStub struct {
	findUser    admin.User
	findErr     error
	findCalls   int
	createCalls int
	createdUser admin.User
	createErr   error
}

func (s *adminUserRepoStub) Create(_ context.Context, user admin.User) error {
	s.createCalls++
	s.createdUser = user
	return s.createErr
}

func (s *adminUserRepoStub) FindByUsername(_ context.Context, username string) (admin.User, error) {
	s.findCalls++
	if username != "FluxDigest" {
		return admin.User{}, errors.New("unexpected username")
	}
	return s.findUser, s.findErr
}

func TestAdminUserServiceSeedDefaultsCreatesDefaultAdminWhenMissing(t *testing.T) {
	repo := &adminUserRepoStub{findErr: admin.ErrNotFound}
	svc := service.NewAdminUserService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	if repo.findCalls != 1 {
		t.Fatalf("want find call 1 got %d", repo.findCalls)
	}
	if repo.createCalls != 1 {
		t.Fatalf("want create call 1 got %d", repo.createCalls)
	}
	if repo.createdUser.Username != "FluxDigest" {
		t.Fatalf("want FluxDigest got %s", repo.createdUser.Username)
	}
	if repo.createdUser.PasswordHash == "FluxDigest" {
		t.Fatal("password hash should not store plain password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.createdUser.PasswordHash), []byte("FluxDigest")); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
	if !repo.createdUser.MustChangePassword {
		t.Fatal("want must_change_password=true")
	}
}

func TestAdminUserServiceSeedDefaultsIsIdempotentWhenDefaultAdminExists(t *testing.T) {
	repo := &adminUserRepoStub{findUser: admin.User{ID: "admin-1", Username: "FluxDigest"}}
	svc := service.NewAdminUserService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	if repo.findCalls != 1 {
		t.Fatalf("want find call 1 got %d", repo.findCalls)
	}
	if repo.createCalls != 0 {
		t.Fatalf("want no create calls got %d", repo.createCalls)
	}
}

func TestAdminUserServiceSeedDefaultsReturnsFindError(t *testing.T) {
	errDBDown := errors.New("db down")
	repo := &adminUserRepoStub{findErr: errDBDown}
	svc := service.NewAdminUserService(repo)

	err := svc.SeedDefaults(context.Background())
	if !errors.Is(err, errDBDown) {
		t.Fatalf("want wrapped find error got %v", err)
	}
	if !strings.Contains(err.Error(), "find default admin user") {
		t.Fatalf("want context prefix in error, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("want no create calls got %d", repo.createCalls)
	}
}

func TestAdminUserServiceSeedDefaultsReturnsCreateError(t *testing.T) {
	errWriteFailed := errors.New("write failed")
	repo := &adminUserRepoStub{findErr: admin.ErrNotFound, createErr: errWriteFailed}
	svc := service.NewAdminUserService(repo)

	err := svc.SeedDefaults(context.Background())
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("want wrapped create error got %v", err)
	}
	if !strings.Contains(err.Error(), "create default admin user") {
		t.Fatalf("want context prefix in error, got %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("want create call 1 got %d", repo.createCalls)
	}
}
