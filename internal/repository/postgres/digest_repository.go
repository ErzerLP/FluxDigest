package postgres

import (
	"context"
	"errors"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/workflow/daily_digest_workflow"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	digestStatePublishing       = "publishing"
	digestStatePublished        = "published"
	digestStateFailed           = "failed"
	digestStateRecoveryRequired = "recovery_required"
)

// DailyDigestRecord 表示日报运行期持久化结果。
type DailyDigestRecord struct {
	ID              string
	DigestDate      string
	Title           string
	Subtitle        string
	ContentMarkdown string
	ContentHTML     string
	RemoteID        string
	RemoteURL       string
	PublishState    string
	PublishError    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DigestRepository 负责保存日报运行结果。
type DigestRepository struct {
	db *gorm.DB
}

// NewDigestRepository 创建 DigestRepository。
func NewDigestRepository(db *gorm.DB) *DigestRepository {
	return &DigestRepository{db: db}
}

// BeginPublish 为日报进入 publishing 状态；首次发布与 failed 重试都会成功。
func (r *DigestRepository) BeginPublish(ctx context.Context, digestDate string, digest daily_digest_workflow.Digest) (bool, error) {
	now := time.Now()
	insertValues := map[string]any{
		"id":               ensureID(""),
		"digest_date":      digestDate,
		"title":            digest.Title,
		"subtitle":         digest.Subtitle,
		"content_markdown": digest.ContentMarkdown,
		"content_html":     digest.ContentHTML,
		"remote_id":        "",
		"remote_url":       "",
		"publish_state":    digestStatePublishing,
		"publish_error":    "",
		"updated_at":       now,
	}

	result := r.db.WithContext(ctx).
		Table(models.DailyDigestModel{}.TableName()).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(insertValues)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 1 {
		return true, nil
	}

	updateValues := map[string]any{
		"title":            digest.Title,
		"subtitle":         digest.Subtitle,
		"content_markdown": digest.ContentMarkdown,
		"content_html":     digest.ContentHTML,
		"remote_id":        "",
		"remote_url":       "",
		"publish_state":    digestStatePublishing,
		"publish_error":    "",
		"updated_at":       now,
	}
	result = r.db.WithContext(ctx).
		Table(models.DailyDigestModel{}.TableName()).
		Where("digest_date = ? AND publish_state = ?", digestDate, digestStateFailed).
		Updates(updateValues)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}

// MarkPublished 将 publishing 结果回写为 published。
func (r *DigestRepository) MarkPublished(ctx context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult) error {
	return r.updatePublishState(ctx, digestDate, map[string]any{
		"remote_id":     publishResult.RemoteID,
		"remote_url":    publishResult.RemoteURL,
		"publish_state": digestStatePublished,
		"publish_error": "",
		"updated_at":    time.Now(),
	})
}

// MarkFailed 将确定未发出的失败写为 failed，允许后续自动重试。
func (r *DigestRepository) MarkFailed(ctx context.Context, digestDate string, publishError string) error {
	return r.updatePublishState(ctx, digestDate, map[string]any{
		"publish_state": digestStateFailed,
		"publish_error": publishError,
		"updated_at":    time.Now(),
	})
}

// MarkRecoveryRequired 将模糊副作用失败写为 recovery_required，必要时保留远端对账信息。
func (r *DigestRepository) MarkRecoveryRequired(ctx context.Context, digestDate string, publishResult adapterpublisher.PublishDigestResult, publishError string) error {
	return r.updatePublishState(ctx, digestDate, map[string]any{
		"remote_id":     publishResult.RemoteID,
		"remote_url":    publishResult.RemoteURL,
		"publish_state": digestStateRecoveryRequired,
		"publish_error": publishError,
		"updated_at":    time.Now(),
	})
}

// GetState 返回日报当前发布状态、远端链接与存在性。
func (r *DigestRepository) GetState(ctx context.Context, digestDate string) (string, string, bool, error) {
	record, err := r.GetByDigestDate(ctx, digestDate)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return record.PublishState, record.RemoteURL, true, nil
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
		RemoteID:        model.RemoteID,
		RemoteURL:       model.RemoteURL,
		PublishState:    model.PublishState,
		PublishError:    model.PublishError,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}, nil
}

func (r *DigestRepository) updatePublishState(ctx context.Context, digestDate string, values map[string]any) error {
	result := r.db.WithContext(ctx).
		Table(models.DailyDigestModel{}.TableName()).
		Where("digest_date = ?", digestDate).
		Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
