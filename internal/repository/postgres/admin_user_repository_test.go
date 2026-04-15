package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"rss-platform/internal/domain/admin"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
)

func TestAdminUserRepositoryCreateAndFindByUsername(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.AdminUserModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewAdminUserRepository(db)
	lastLoginAt := time.Date(2026, 4, 13, 8, 0, 0, 0, time.UTC)

	err := repo.Create(context.Background(), admin.User{
		Username:           "FluxDigest",
		PasswordHash:       "bcrypt-hash",
		MustChangePassword: true,
		LastLoginAt:        lastLoginAt,
	})
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	got, err := repo.FindByUsername(context.Background(), "FluxDigest")
	if err != nil {
		t.Fatalf("find by username: %v", err)
	}
	if got.ID == "" {
		t.Fatal("want generated id, got empty")
	}
	if got.Username != "FluxDigest" {
		t.Fatalf("want FluxDigest got %s", got.Username)
	}
	if got.PasswordHash != "bcrypt-hash" {
		t.Fatalf("want bcrypt-hash got %s", got.PasswordHash)
	}
	if !got.MustChangePassword {
		t.Fatal("want must_change_password=true")
	}
	if !got.LastLoginAt.Equal(lastLoginAt) {
		t.Fatalf("want %s got %s", lastLoginAt, got.LastLoginAt)
	}
}

func TestAdminUserRepositoryFindByUsernameReturnsDomainErrNotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.AdminUserModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewAdminUserRepository(db)
	_, err := repo.FindByUsername(context.Background(), "missing")
	if !errors.Is(err, admin.ErrNotFound) {
		t.Fatalf("want admin.ErrNotFound got %v", err)
	}
}
