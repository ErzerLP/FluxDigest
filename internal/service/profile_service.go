package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	promptassets "rss-platform/configs/prompts"
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
	promptsPayload, err := defaultPromptsPayload()
	if err != nil {
		return fmt.Errorf("build default prompts payload: %w", err)
	}

	defaults := []struct {
		profileType string
		name        string
		payload     map[string]any
	}{
		{
			profileType: profile.TypeLLM,
			name:        "default-llm",
			payload: map[string]any{
				"base_url":        "",
				"model":           "MiniMax-M2.7",
				"fallback_models": []string{"mimo-v2-pro"},
				"timeout_ms":      30000,
				"is_enabled":      true,
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
			payload:     promptsPayload,
		},
		{
			profileType: profile.TypePublish,
			name:        "default-publish",
			payload: map[string]any{
				"provider":             "halo",
				"halo_base_url":        "",
				"halo_token":           "",
				"output_dir":           "",
				"article_publish_mode": articlePublishModeDigestOnly,
				"article_review_mode":  articleReviewModeManualReview,
				"is_enabled":           true,
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

func defaultPromptsPayload() (map[string]any, error) {
	translationPrompt, err := promptassets.Read("translation.tmpl")
	if err != nil {
		return nil, err
	}
	analysisPrompt, err := promptassets.Read("analysis.tmpl")
	if err != nil {
		return nil, err
	}
	dossierPrompt, err := promptassets.Read("dossier.tmpl")
	if err != nil {
		return nil, err
	}
	digestPrompt, err := promptassets.Read("digest.tmpl")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"target_language":    "zh-CN",
		"translation_prompt": translationPrompt,
		"analysis_prompt":    analysisPrompt,
		"dossier_prompt":     dossierPrompt,
		"digest_prompt":      digestPrompt,
		"is_enabled":         true,
	}, nil
}
