package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"rss-platform/internal/domain/admin"
)

const (
	defaultAdminUsername = "FluxDigest"
	defaultAdminPassword = "FluxDigest"
)

type AdminBootstrapConfig struct {
	Username string
	Password string
}

type AdminUserRepository interface {
	Create(ctx context.Context, user admin.User) error
	FindByUsername(ctx context.Context, username string) (admin.User, error)
}

type AdminUserService struct {
	repo      AdminUserRepository
	bootstrap AdminBootstrapConfig
}

func NewAdminUserService(repo AdminUserRepository, bootstrap AdminBootstrapConfig) *AdminUserService {
	return &AdminUserService{
		repo:      repo,
		bootstrap: bootstrap,
	}
}

func (s *AdminUserService) SeedDefaults(ctx context.Context) error {
	username := strings.TrimSpace(s.bootstrap.Username)
	if username == "" {
		username = defaultAdminUsername
	}
	password := strings.TrimSpace(s.bootstrap.Password)
	if password == "" {
		password = defaultAdminPassword
	}

	if _, err := s.repo.FindByUsername(ctx, username); err == nil {
		return nil
	} else if !errors.Is(err, admin.ErrNotFound) {
		return fmt.Errorf("find default admin user: %w", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}

	if err := s.repo.Create(ctx, admin.User{
		Username:           username,
		PasswordHash:       string(passwordHash),
		MustChangePassword: true,
	}); err != nil {
		return fmt.Errorf("create default admin user: %w", err)
	}

	return nil
}
