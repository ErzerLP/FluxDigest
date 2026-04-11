package service

import (
	"context"
	"encoding/json"
	"errors"

	"rss-platform/internal/domain/profile"
	postgresrepo "rss-platform/internal/repository/postgres"

	"gorm.io/gorm"
)

type activeProfileReader interface {
	GetActive(ctx context.Context, profileType string) (profile.Version, error)
}

// ProfileView 表示对外暴露的活动配置。
type ProfileView struct {
	ProfileType string         `json:"profile_type"`
	Name        string         `json:"name"`
	Version     int            `json:"version"`
	IsActive    bool           `json:"is_active"`
	Payload     map[string]any `json:"payload"`
}

// ProfileQueryService 负责读取活动配置。
type ProfileQueryService struct {
	profiles activeProfileReader
}

// NewProfileQueryService 创建 ProfileQueryService。
func NewProfileQueryService(db *gorm.DB) *ProfileQueryService {
	svc := &ProfileQueryService{}
	if db != nil {
		svc.profiles = postgresrepo.NewProfileRepository(db)
	}
	return svc
}

// ActiveProfile 返回当前活动配置；缺失时返回空 payload。
func (s *ProfileQueryService) ActiveProfile(ctx context.Context, profileType string) (ProfileView, error) {
	empty := ProfileView{
		ProfileType: profileType,
		Payload:     map[string]any{},
	}
	if s == nil || s.profiles == nil {
		return empty, nil
	}

	version, err := s.profiles.GetActive(ctx, profileType)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return empty, nil
		}
		return ProfileView{}, err
	}

	payload := map[string]any{}
	if len(version.PayloadJSON) > 0 {
		if err := json.Unmarshal(version.PayloadJSON, &payload); err != nil {
			return ProfileView{}, err
		}
	}

	return ProfileView{
		ProfileType: version.ProfileType,
		Name:        version.Name,
		Version:     version.Version,
		IsActive:    version.IsActive,
		Payload:     payload,
	}, nil
}
