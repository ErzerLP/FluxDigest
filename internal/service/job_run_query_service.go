package service

import (
	"context"
	"encoding/json"
	"errors"

	postgresrepo "rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
)

// JobRunRecord 表示作业运行记录。
type JobRunRecord = postgresrepo.JobRunRecord

// JobRunListFilter 表示作业查询过滤条件。
type JobRunListFilter = postgresrepo.JobRunListFilter

// JobRunQueryService 负责读取作业运行记录。
type JobRunQueryService struct {
	db *gorm.DB
}

// NewJobRunQueryService 创建 JobRunQueryService。
func NewJobRunQueryService(db *gorm.DB) *JobRunQueryService {
	return &JobRunQueryService{db: db}
}

// ListLatest 返回最新的作业运行记录。
func (s *JobRunQueryService) ListLatest(ctx context.Context, filter JobRunListFilter) ([]JobRunRecord, error) {
	if s == nil || s.db == nil {
		return []JobRunRecord{}, nil
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	var modelsList []models.JobRunModel
	err := s.db.WithContext(ctx).
		Order("requested_at DESC").
		Order("id DESC").
		Limit(limit).
		Find(&modelsList).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []JobRunRecord{}, nil
		}
		return nil, err
	}

	records := make([]JobRunRecord, 0, len(modelsList))
	for _, model := range modelsList {
		record, err := mapJobRunModelToRecord(model)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

func mapJobRunModelToRecord(model models.JobRunModel) (JobRunRecord, error) {
	detail := map[string]any{}
	if len(model.DetailJSON) > 0 {
		if err := json.Unmarshal(model.DetailJSON, &detail); err != nil {
			return JobRunRecord{}, err
		}
	}

	digestDate := ""
	if model.DigestDate != nil {
		digestDate = model.DigestDate.Format("2006-01-02")
	}

	record := JobRunRecord{
		ID:            model.ID,
		JobType:       model.JobType,
		TriggerSource: model.TriggerSource,
		Status:        model.Status,
		DigestDate:    digestDate,
		Detail:        detail,
		ErrorMessage:  model.ErrorMessage,
		RequestedAt:   model.RequestedAt,
	}

	if model.StartedAt != nil {
		record.StartedAt = *model.StartedAt
	}
	if model.FinishedAt != nil {
		record.FinishedAt = *model.FinishedAt
	}

	return record, nil
}
