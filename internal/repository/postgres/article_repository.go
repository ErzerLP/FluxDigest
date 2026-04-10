package postgres

import (
	"context"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ArticleRepository struct {
	db *gorm.DB
}

func NewArticleRepository(db *gorm.DB) *ArticleRepository {
	return &ArticleRepository{db: db}
}

func (r *ArticleRepository) Upsert(ctx context.Context, a article.SourceArticle) error {
	m := models.SourceArticleModel{
		ID:              a.ID,
		MinifluxEntryID: a.MinifluxEntryID,
		Title:           a.Title,
		URL:             a.URL,
		Fingerprint:     a.Fingerprint,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "miniflux_entry_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "url", "fingerprint"}),
		}).
		Create(&m).Error
}

func (r *ArticleRepository) FindByMinifluxEntryID(ctx context.Context, minifluxEntryID int64) (article.SourceArticle, error) {
	var m models.SourceArticleModel
	if err := r.db.WithContext(ctx).Where("miniflux_entry_id = ?", minifluxEntryID).First(&m).Error; err != nil {
		return article.SourceArticle{}, err
	}

	return article.SourceArticle{
		ID:              m.ID,
		MinifluxEntryID: m.MinifluxEntryID,
		Title:           m.Title,
		URL:             m.URL,
		Fingerprint:     m.Fingerprint,
	}, nil
}
