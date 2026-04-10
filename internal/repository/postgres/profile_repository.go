package postgres

import (
	"context"

	"rss-platform/internal/domain/profile"
	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
)

type ProfileRepository struct {
	db *gorm.DB
}

func NewProfileRepository(db *gorm.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) Create(ctx context.Context, v profile.Version) error {
	m := models.ProfileVersionModel{
		ID:          v.ID,
		ProfileType: v.ProfileType,
		Name:        v.Name,
		Version:     v.Version,
		IsActive:    v.IsActive,
		PayloadJSON: v.PayloadJSON,
	}

	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *ProfileRepository) GetActive(ctx context.Context, profileType string) (profile.Version, error) {
	var m models.ProfileVersionModel
	if err := r.db.WithContext(ctx).Where("profile_type = ? AND is_active = ?", profileType, true).First(&m).Error; err != nil {
		return profile.Version{}, err
	}

	return profile.Version{
		ID:          m.ID,
		ProfileType: m.ProfileType,
		Name:        m.Name,
		Version:     m.Version,
		IsActive:    m.IsActive,
		PayloadJSON: m.PayloadJSON,
	}, nil
}
