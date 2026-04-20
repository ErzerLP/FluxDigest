package postgres

import (
	"context"
	"encoding/json"
	"time"

	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
)

// JobRunRecord 表示作业运行记录。
type JobRunRecord struct {
	ID            string
	JobType       string
	TriggerSource string
	Status        string
	DigestDate    string
	Detail        map[string]any
	ErrorMessage  string
	RequestedAt   time.Time
	StartedAt     time.Time
	FinishedAt    time.Time
}

// JobRunListFilter 表示作业查询过滤条件。
type JobRunListFilter struct {
	Limit int
}

// JobRunRepository 负责保存作业运行记录。
type JobRunRepository struct {
	db *gorm.DB
}

// NewJobRunRepository 创建 JobRunRepository。
func NewJobRunRepository(db *gorm.DB) *JobRunRepository {
	return &JobRunRepository{db: db}
}

// Create 持久化一次作业运行记录。
func (r *JobRunRepository) Create(ctx context.Context, record JobRunRecord) error {
	detail := record.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return err
	}

	var digestDate *time.Time
	if record.DigestDate != "" {
		parsed, err := time.Parse("2006-01-02", record.DigestDate)
		if err != nil {
			return err
		}
		digestDate = &parsed
	}

	model := models.JobRunModel{
		ID:            ensureID(record.ID),
		JobType:       record.JobType,
		TriggerSource: record.TriggerSource,
		Status:        record.Status,
		DigestDate:    digestDate,
		DetailJSON:    detailJSON,
		ErrorMessage:  record.ErrorMessage,
		RequestedAt:   record.RequestedAt,
	}

	if !record.StartedAt.IsZero() {
		model.StartedAt = &record.StartedAt
	}
	if !record.FinishedAt.IsZero() {
		model.FinishedAt = &record.FinishedAt
	}

	return r.db.WithContext(ctx).Create(&model).Error
}

// ListLatest 返回最新的作业运行记录。
func (r *JobRunRepository) ListLatest(ctx context.Context, filter JobRunListFilter) ([]JobRunRecord, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	var modelsList []models.JobRunModel
	if err := r.db.WithContext(ctx).
		Order("requested_at DESC").
		Order("id DESC").
		Limit(limit).
		Find(&modelsList).Error; err != nil {
		return nil, err
	}

	records := make([]JobRunRecord, 0, len(modelsList))
	for _, model := range modelsList {
		record, err := mapJobRunModel(model)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

// LatestByType 返回指定 job_type 最新的一条记录。
func (r *JobRunRepository) LatestByType(ctx context.Context, jobType string) (JobRunRecord, error) {
	var model models.JobRunModel
	if err := r.db.WithContext(ctx).
		Where("job_type = ?", jobType).
		Order("requested_at DESC").
		Order("id DESC").
		Take(&model).Error; err != nil {
		return JobRunRecord{}, err
	}

	return mapJobRunModel(model)
}

func mapJobRunModel(model models.JobRunModel) (JobRunRecord, error) {
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
