package postgres

import (
	"context"
	"time"

	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ArticlePublishStateRecord 表示 dossier 发布状态记录。
type ArticlePublishStateRecord struct {
	ID             string
	DossierID      string
	State          string
	ApprovedBy     string
	DecisionNote   string
	PublishChannel string
	RemoteID       string
	RemoteURL      string
	ErrorMessage   string
	PublishedAt    *time.Time
	UpdatedAt      time.Time
}

// PublishStateRepository 负责管理发布状态。
type PublishStateRepository struct {
	db *gorm.DB
}

// NewPublishStateRepository 创建 PublishStateRepository。
func NewPublishStateRepository(db *gorm.DB) *PublishStateRepository {
	return &PublishStateRepository{db: db}
}

// Upsert 按 dossier_id 写入或更新发布状态。
func (r *PublishStateRepository) Upsert(ctx context.Context, input ArticlePublishStateRecord) error {
	model := models.ArticlePublishStateModel{
		ID:             ensureID(input.ID),
		DossierID:      input.DossierID,
		State:          input.State,
		ApprovedBy:     input.ApprovedBy,
		DecisionNote:   input.DecisionNote,
		PublishChannel: input.PublishChannel,
		RemoteID:       input.RemoteID,
		RemoteURL:      input.RemoteURL,
		ErrorMessage:   input.ErrorMessage,
		PublishedAt:    input.PublishedAt,
		UpdatedAt:      defaultTime(input.UpdatedAt, time.Now()),
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "dossier_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"state",
				"approved_by",
				"decision_note",
				"publish_channel",
				"remote_id",
				"remote_url",
				"error_message",
				"published_at",
				"updated_at",
			}),
		}).
		Create(&model).Error
}

// GetByDossierID 读取指定 dossier 的发布状态。
func (r *PublishStateRepository) GetByDossierID(ctx context.Context, dossierID string) (ArticlePublishStateRecord, error) {
	var model models.ArticlePublishStateModel
	if err := r.db.WithContext(ctx).Where("dossier_id = ?", dossierID).First(&model).Error; err != nil {
		return ArticlePublishStateRecord{}, err
	}

	return ArticlePublishStateRecord{
		ID:             model.ID,
		DossierID:      model.DossierID,
		State:          model.State,
		ApprovedBy:     model.ApprovedBy,
		DecisionNote:   model.DecisionNote,
		PublishChannel: model.PublishChannel,
		RemoteID:       model.RemoteID,
		RemoteURL:      model.RemoteURL,
		ErrorMessage:   model.ErrorMessage,
		PublishedAt:    model.PublishedAt,
		UpdatedAt:      model.UpdatedAt,
	}, nil
}
