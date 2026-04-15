package postgres

import (
	"context"
	"errors"

	"rss-platform/internal/domain/admin"
	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
)

type AdminUserRepository struct {
	db *gorm.DB
}

func NewAdminUserRepository(db *gorm.DB) *AdminUserRepository {
	return &AdminUserRepository{db: db}
}

func (r *AdminUserRepository) Create(ctx context.Context, user admin.User) error {
	model := models.AdminUserModel{
		ID:                 ensureID(user.ID),
		Username:           user.Username,
		PasswordHash:       user.PasswordHash,
		MustChangePassword: user.MustChangePassword,
	}
	if !user.LastLoginAt.IsZero() {
		model.LastLoginAt = &user.LastLoginAt
	}

	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *AdminUserRepository) FindByUsername(ctx context.Context, username string) (admin.User, error) {
	var model models.AdminUserModel
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return admin.User{}, admin.ErrNotFound
		}
		return admin.User{}, err
	}

	user := admin.User{
		ID:                 model.ID,
		Username:           model.Username,
		PasswordHash:       model.PasswordHash,
		MustChangePassword: model.MustChangePassword,
	}
	if model.LastLoginAt != nil {
		user.LastLoginAt = *model.LastLoginAt
	}

	return user, nil
}
