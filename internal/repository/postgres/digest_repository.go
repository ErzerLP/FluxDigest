package postgres

import (
	"context"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/workflow/daily_digest_workflow"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DailyDigestRecord 表示日报运行期持久化结果。
type DailyDigestRecord struct {
	ID              string
	DigestDate      string
	Title           string
	Subtitle        string
	ContentMarkdown string
	ContentHTML     string
	RemoteURL       string
	CreatedAt       time.Time
}

// DigestRepository 负责保存日报运行结果。
type DigestRepository struct {
	db *gorm.DB
}

// NewDigestRepository 创建 DigestRepository。
func NewDigestRepository(db *gorm.DB) *DigestRepository {
	return &DigestRepository{db: db}
}

// Save 按稳定日语义幂等写入日报结果。
func (r *DigestRepository) Save(ctx context.Context, runAt time.Time, digest daily_digest_workflow.Digest, publishResult adapterpublisher.PublishDigestResult) error {
	digestDate := runAt.Format("2006-01-02")
	values := map[string]any{
		"id":               ensureID(""),
		"digest_date":      digestDate,
		"title":            digest.Title,
		"subtitle":         digest.Subtitle,
		"content_markdown": digest.ContentMarkdown,
		"content_html":     digest.ContentHTML,
		"remote_url":       publishResult.RemoteURL,
	}

	return r.db.WithContext(ctx).
		Table(models.DailyDigestModel{}.TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "digest_date"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"title",
				"subtitle",
				"content_markdown",
				"content_html",
				"remote_url",
			}),
		}).
		Create(values).Error
}

// GetByDigestDate 按日期读取已保存的日报结果。
func (r *DigestRepository) GetByDigestDate(ctx context.Context, digestDate string) (DailyDigestRecord, error) {
	var model models.DailyDigestModel
	if err := r.db.WithContext(ctx).
		Where("digest_date = ?", digestDate).
		First(&model).Error; err != nil {
		return DailyDigestRecord{}, err
	}

	return DailyDigestRecord{
		ID:              model.ID,
		DigestDate:      model.DigestDate.Format("2006-01-02"),
		Title:           model.Title,
		Subtitle:        model.Subtitle,
		ContentMarkdown: model.ContentMarkdown,
		ContentHTML:     model.ContentHTML,
		RemoteURL:       model.RemoteURL,
		CreatedAt:       model.CreatedAt,
	}, nil
}
