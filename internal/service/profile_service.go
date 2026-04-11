package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"rss-platform/internal/domain/profile"
)

type ProfileRepository interface {
	Create(ctx context.Context, v profile.Version) error
	GetActive(ctx context.Context, profileType string) (profile.Version, error)
}

type ProfileService struct {
	repo ProfileRepository
}

func NewProfileService(repo ProfileRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

func (s *ProfileService) SeedDefaults(ctx context.Context) error {
	defaults := []struct {
		profileType string
		name        string
		payload     map[string]any
	}{
		{
			profileType: profile.TypeLLM,
			name:        "default-llm",
			payload: map[string]any{
				"base_url":   "",
				"model":      "gpt-4.1-mini",
				"timeout_ms": 30000,
				"is_enabled": true,
			},
		},
		{
			profileType: profile.TypeMiniflux,
			name:        "default-miniflux",
			payload: map[string]any{
				"base_url":       "",
				"api_token":      "",
				"fetch_limit":    100,
				"lookback_hours": 24,
				"is_enabled":     true,
			},
		},
		{
			profileType: profile.TypePrompts,
			name:        "default-prompts",
			payload: map[string]any{
				"target_language":    "zh-CN",
				"translation_prompt": "",
				"analysis_prompt":    "",
				"digest_prompt":      "",
				"is_enabled":         true,
			},
		},
		{
			profileType: profile.TypePublish,
			name:        "default-publish",
			payload: map[string]any{
				"target_type":    "holo",
				"endpoint":       "",
				"auth_token":     "",
				"content_format": "markdown",
				"is_enabled":     true,
			},
		},
		{
			profileType: profile.TypeScheduler,
			name:        "default-scheduler",
			payload: map[string]any{
				"schedule_enabled": true,
				"schedule_time":    "07:00",
				"timezone":         "Asia/Shanghai",
			},
		},
	}

	for _, def := range defaults {
		if _, err := s.repo.GetActive(ctx, def.profileType); err == nil {
			continue
		} else if !errors.Is(err, profile.ErrNotFound) {
			return fmt.Errorf("check active %s profile: %w", def.profileType, err)
		}

		payload, err := json.Marshal(def.payload)
		if err != nil {
			return fmt.Errorf("marshal default %s profile payload: %w", def.profileType, err)
		}
		if err := s.repo.Create(ctx, profile.Version{
			ProfileType: def.profileType,
			Name:        def.name,
			Version:     1,
			IsActive:    true,
			PayloadJSON: payload,
		}); err != nil {
			return fmt.Errorf("create default %s profile: %w", def.profileType, err)
		}
	}

	return nil
}
