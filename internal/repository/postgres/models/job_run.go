package models

import "time"

// JobRunModel 表示作业运行记录。
type JobRunModel struct {
	ID            string     `gorm:"primaryKey;size:36"`
	JobType       string     `gorm:"not null;index:idx_job_runs_type_requested_at,priority:1"`
	TriggerSource string     `gorm:"not null;default:''"`
	Status        string     `gorm:"not null"`
	DigestDate    *time.Time `gorm:"type:date"`
	DetailJSON    []byte     `gorm:"type:jsonb;not null;default:'{}'"`
	ErrorMessage  string     `gorm:"not null;default:''"`
	RequestedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;autoCreateTime"`
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

func (JobRunModel) TableName() string {
	return "job_runs"
}
