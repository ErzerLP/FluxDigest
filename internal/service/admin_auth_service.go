package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"rss-platform/internal/domain/admin"
)

const (
	DefaultAdminSessionCookieName = "fluxdigest_admin_session"
	defaultAdminSessionTTL        = 24 * time.Hour
	defaultAdminSessionKeyPrefix  = "fluxdigest:admin:sessions:"
)

var ErrInvalidAdminCredentials = errors.New("invalid admin credentials")
var ErrAdminSessionNotFound = errors.New("admin session not found")

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	UserID             string `json:"user_id"`
	Username           string `json:"username"`
	MustChangePassword bool   `json:"must_change_password"`
}

type AdminSession struct {
	UserID             string    `json:"user_id"`
	Username           string    `json:"username"`
	MustChangePassword bool      `json:"must_change_password"`
	IssuedAt           time.Time `json:"issued_at"`
}

type AdminSessionStore interface {
	Save(ctx context.Context, sessionID string, session AdminSession, ttl time.Duration) error
	Load(ctx context.Context, sessionID string) (AdminSession, error)
	Delete(ctx context.Context, sessionID string) error
}

type AdminAuthOption func(*AdminAuthService)

type AdminAuthService struct {
	repo         AdminUserRepository
	sessions     AdminSessionStore
	sessionTTL   time.Duration
	now          func() time.Time
	newSessionID func() (string, error)
}

func NewAdminAuthService(repo AdminUserRepository, sessions AdminSessionStore, options ...AdminAuthOption) *AdminAuthService {
	svc := &AdminAuthService{
		repo:       repo,
		sessions:   sessions,
		sessionTTL: defaultAdminSessionTTL,
		now:        time.Now,
		newSessionID: func() (string, error) {
			buf := make([]byte, 32)
			if _, err := rand.Read(buf); err != nil {
				return "", err
			}
			return base64.RawURLEncoding.EncodeToString(buf), nil
		},
	}
	for _, option := range options {
		if option != nil {
			option(svc)
		}
	}
	return svc
}

func WithAdminSessionTTL(ttl time.Duration) AdminAuthOption {
	return func(s *AdminAuthService) {
		if ttl > 0 {
			s.sessionTTL = ttl
		}
	}
}

func (s *AdminAuthService) SessionTTL() time.Duration {
	if s == nil || s.sessionTTL <= 0 {
		return defaultAdminSessionTTL
	}
	return s.sessionTTL
}

func (s *AdminAuthService) Login(ctx context.Context, input LoginInput) (LoginResult, string, error) {
	if s == nil || s.repo == nil || s.sessions == nil {
		return LoginResult{}, "", errors.New("admin auth service is not configured")
	}

	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		return LoginResult{}, "", ErrInvalidAdminCredentials
	}
	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			return LoginResult{}, "", ErrInvalidAdminCredentials
		}
		return LoginResult{}, "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, "", ErrInvalidAdminCredentials
	}

	result := LoginResult{
		UserID:             user.ID,
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
	}
	sessionID, err := s.newSessionID()
	if err != nil {
		return LoginResult{}, "", fmt.Errorf("generate admin session id: %w", err)
	}
	if err := s.sessions.Save(ctx, sessionID, AdminSession{
		UserID:             result.UserID,
		Username:           result.Username,
		MustChangePassword: result.MustChangePassword,
		IssuedAt:           s.now().UTC(),
	}, s.SessionTTL()); err != nil {
		return LoginResult{}, "", err
	}

	return result, sessionID, nil
}

func (s *AdminAuthService) Logout(ctx context.Context, sessionID string) error {
	if s == nil || s.sessions == nil {
		return errors.New("admin auth service is not configured")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := s.sessions.Delete(ctx, sessionID); err != nil && !errors.Is(err, ErrAdminSessionNotFound) {
		return err
	}
	return nil
}

func (s *AdminAuthService) CurrentUser(ctx context.Context, sessionID string) (LoginResult, error) {
	if s == nil || s.sessions == nil {
		return LoginResult{}, errors.New("admin auth service is not configured")
	}
	if strings.TrimSpace(sessionID) == "" {
		return LoginResult{}, ErrAdminSessionNotFound
	}

	session, err := s.sessions.Load(ctx, sessionID)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		UserID:             session.UserID,
		Username:           session.Username,
		MustChangePassword: session.MustChangePassword,
	}, nil
}

type InMemoryAdminSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]inMemoryAdminSession
}

type inMemoryAdminSession struct {
	session   AdminSession
	expiresAt time.Time
}

func NewInMemoryAdminSessionStore() *InMemoryAdminSessionStore {
	return &InMemoryAdminSessionStore{sessions: make(map[string]inMemoryAdminSession)}
}

func (s *InMemoryAdminSessionStore) Save(_ context.Context, sessionID string, session AdminSession, ttl time.Duration) error {
	if s == nil {
		return errors.New("memory admin session store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = inMemoryAdminSession{session: session, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *InMemoryAdminSessionStore) Load(_ context.Context, sessionID string) (AdminSession, error) {
	if s == nil {
		return AdminSession{}, errors.New("memory admin session store is nil")
	}

	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return AdminSession{}, ErrAdminSessionNotFound
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return AdminSession{}, ErrAdminSessionNotFound
	}
	return entry.session, nil
}

func (s *InMemoryAdminSessionStore) Delete(_ context.Context, sessionID string) error {
	if s == nil {
		return errors.New("memory admin session store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

type RedisAdminSessionStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisAdminSessionStore(addr string) *RedisAdminSessionStore {
	prefix := defaultAdminSessionKeyPrefix
	return &RedisAdminSessionStore{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		prefix: prefix,
	}
}

func (s *RedisAdminSessionStore) Save(ctx context.Context, sessionID string, session AdminSession, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return errors.New("redis admin session store is not configured")
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal admin session: %w", err)
	}
	if err := s.client.Set(ctx, s.key(sessionID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save admin session: %w", err)
	}
	return nil
}

func (s *RedisAdminSessionStore) Load(ctx context.Context, sessionID string) (AdminSession, error) {
	if s == nil || s.client == nil {
		return AdminSession{}, errors.New("redis admin session store is not configured")
	}
	payload, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return AdminSession{}, ErrAdminSessionNotFound
		}
		return AdminSession{}, fmt.Errorf("load admin session: %w", err)
	}
	var session AdminSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return AdminSession{}, fmt.Errorf("unmarshal admin session: %w", err)
	}
	return session, nil
}

func (s *RedisAdminSessionStore) Delete(ctx context.Context, sessionID string) error {
	if s == nil || s.client == nil {
		return errors.New("redis admin session store is not configured")
	}
	if err := s.client.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}

func (s *RedisAdminSessionStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisAdminSessionStore) key(sessionID string) string {
	prefix := s.prefix
	if prefix == "" {
		prefix = defaultAdminSessionKeyPrefix
	}
	return prefix + sessionID
}
