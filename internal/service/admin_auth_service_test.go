package service_test

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"rss-platform/internal/domain/admin"
	"rss-platform/internal/service"
)

type adminAuthRepoStub struct {
	user      admin.User
	err       error
	findCalls int
}

func (s *adminAuthRepoStub) Create(context.Context, admin.User) error {
	return errors.New("unexpected create call")
}

func (s *adminAuthRepoStub) FindByUsername(_ context.Context, username string) (admin.User, error) {
	s.findCalls++
	if username != s.user.Username {
		return admin.User{}, admin.ErrNotFound
	}
	if s.err != nil {
		return admin.User{}, s.err
	}
	return s.user, nil
}

func TestAdminAuthServiceLoginRequiresPasswordChangeFlag(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("FluxDigest"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &adminAuthRepoStub{
		user: admin.User{
			ID:                 "admin-1",
			Username:           "FluxDigest",
			PasswordHash:       string(passwordHash),
			MustChangePassword: true,
		},
	}
	store := service.NewInMemoryAdminSessionStore()
	svc := service.NewAdminAuthService(repo, store)

	result, sessionID, err := svc.Login(context.Background(), service.LoginInput{
		Username: "FluxDigest",
		Password: "FluxDigest",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sessionID == "" {
		t.Fatal("want non-empty session id")
	}
	if result.UserID != "admin-1" {
		t.Fatalf("want user id admin-1 got %q", result.UserID)
	}
	if result.Username != "FluxDigest" {
		t.Fatalf("want username FluxDigest got %q", result.Username)
	}
	if !result.MustChangePassword {
		t.Fatal("want must_change_password=true")
	}

	current, err := svc.CurrentUser(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if current != result {
		t.Fatalf("want current user %+v got %+v", result, current)
	}
}

func TestAdminAuthServiceLoginRejectsBlankCredentialsAsInvalid(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("FluxDigest"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name     string
		input    service.LoginInput
		wantCall int
	}{
		{
			name:     "blank username",
			input:    service.LoginInput{Username: "   ", Password: "FluxDigest"},
			wantCall: 0,
		},
		{
			name:     "blank password",
			input:    service.LoginInput{Username: "FluxDigest", Password: "   "},
			wantCall: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &adminAuthRepoStub{
				user: admin.User{
					ID:                 "admin-1",
					Username:           "FluxDigest",
					PasswordHash:       string(passwordHash),
					MustChangePassword: true,
				},
			}
			svc := service.NewAdminAuthService(repo, service.NewInMemoryAdminSessionStore())

			_, _, err := svc.Login(context.Background(), tc.input)
			if !errors.Is(err, service.ErrInvalidAdminCredentials) {
				t.Fatalf("want invalid credentials got %v", err)
			}
			if repo.findCalls != tc.wantCall {
				t.Fatalf("want find calls %d got %d", tc.wantCall, repo.findCalls)
			}
		})
	}
}
