package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"rss-platform/internal/domain/profile"
)

var ErrProfileNotFound = errors.New("profile active version not found")

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
			profileType: "ai",
			name:        "default-ai",
			payload: map[string]any{
				"provider":                    "openai",
				"model":                       "gpt-4.1-mini",
				"temperature":                 0.2,
				"translation_prompt_template": "configs/prompts/translation.tmpl",
				"analysis_prompt_template":    "configs/prompts/analysis.tmpl",
			},
		},
		{
			profileType: "digest",
			name:        "default-digest",
			payload: map[string]any{
				"prompt_path": "configs/prompts/digest.tmpl",
				"max_items":   10,
			},
		},
		{
			profileType: "publish",
			name:        "default-publish",
			payload: map[string]any{
				"channel": "markdown",
				"enabled": true,
			},
		},
		{
			profileType: "api",
			name:        "default-api",
			payload: map[string]any{
				"timeout_seconds": 30,
				"retry":           1,
			},
		},
	}

	for _, def := range defaults {
		if _, err := s.repo.GetActive(ctx, def.profileType); err == nil {
			continue
		} else if !errors.Is(err, ErrProfileNotFound) {
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
