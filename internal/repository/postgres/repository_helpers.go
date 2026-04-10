package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ensureID(id string) string {
	if id == "" {
		return uuid.NewString()
	}
	return id
}

func withTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}
