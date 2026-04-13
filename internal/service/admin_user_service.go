package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"rss-platform/internal/domain/admin"
)

const (
	defaultAdminUsername = "FluxDigest"
	defaultAdminPassword = "FluxDigest"
)

type AdminUserRepository interface {
	Create(ctx context.Context, user admin.User) error
	FindByUsername(ctx context.Context, username string) (admin.User, error)
}

type AdminUserService struct {
	repo AdminUserRepository
}

func NewAdminUserService(repo AdminUserRepository) *AdminUserService {
	return &AdminUserService{repo: repo}
}

func (s *AdminUserService) SeedDefaults(ctx context.Context) error {
	if _, err := s.repo.FindByUsername(ctx, defaultAdminUsername); err == nil {
		return nil
	} else if !errors.Is(err, admin.ErrNotFound) {
		return fmt.Errorf("find default admin user: %w", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}

	if err := s.repo.Create(ctx, admin.User{
		Username:           defaultAdminUsername,
		PasswordHash:       string(passwordHash),
		MustChangePassword: true,
	}); err != nil {
		return fmt.Errorf("create default admin user: %w", err)
	}

	return nil
}
