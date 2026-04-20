package service

import (
	"context"
	"time"

	postgresrepo "rss-platform/internal/repository/postgres"

	"gorm.io/gorm"
)

// JobRunRecord 表示作业运行记录。
type JobRunRecord struct {
	ID            string         `json:"id"`
	JobType       string         `json:"job_type"`
	TriggerSource string         `json:"trigger_source"`
	Status        string         `json:"status"`
	DigestDate    string         `json:"digest_date"`
	Detail        map[string]any `json:"detail"`
	ErrorMessage  string         `json:"error_message"`
	RequestedAt   time.Time      `json:"requested_at"`
	StartedAt     time.Time      `json:"started_at"`
	FinishedAt    time.Time      `json:"finished_at"`
}

// JobRunListFilter 表示作业查询过滤条件。
type JobRunListFilter struct {
	Limit int
}

type jobRunReader interface {
	ListLatest(ctx context.Context, filter JobRunListFilter) ([]JobRunRecord, error)
}

// JobRunQueryService 负责读取作业运行记录。
type JobRunQueryService struct {
	reader jobRunReader
}

// NewJobRunQueryService 创建 JobRunQueryService。
func NewJobRunQueryService(db *gorm.DB) *JobRunQueryService {
	svc := &JobRunQueryService{}
	if db != nil {
		svc.reader = &jobRunRepoAdapter{repo: postgresrepo.NewJobRunRepository(db)}
	}
	return svc
}

// ListLatest 返回最新的作业运行记录。
func (s *JobRunQueryService) ListLatest(ctx context.Context, filter JobRunListFilter) ([]JobRunRecord, error) {
	if s == nil || s.reader == nil {
		return []JobRunRecord{}, nil
	}
	return s.reader.ListLatest(ctx, filter)
}

type jobRunRepoAdapter struct {
	repo *postgresrepo.JobRunRepository
}

func (a *jobRunRepoAdapter) ListLatest(ctx context.Context, filter JobRunListFilter) ([]JobRunRecord, error) {
	if a == nil || a.repo == nil {
		return []JobRunRecord{}, nil
	}

	records, err := a.repo.ListLatest(ctx, postgresrepo.JobRunListFilter{Limit: filter.Limit})
	if err != nil {
		return nil, err
	}

	out := make([]JobRunRecord, 0, len(records))
	for _, record := range records {
		out = append(out, mapJobRunRecord(record))
	}

	return out, nil
}

func mapJobRunRecord(record postgresrepo.JobRunRecord) JobRunRecord {
	return JobRunRecord{
		ID:            record.ID,
		JobType:       record.JobType,
		TriggerSource: record.TriggerSource,
		Status:        record.Status,
		DigestDate:    record.DigestDate,
		Detail:        record.Detail,
		ErrorMessage:  record.ErrorMessage,
		RequestedAt:   record.RequestedAt,
		StartedAt:     record.StartedAt,
		FinishedAt:    record.FinishedAt,
	}
}
