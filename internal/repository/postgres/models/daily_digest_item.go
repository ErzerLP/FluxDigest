package models

import "time"

type DailyDigestItemModel struct {
	ID               string    `gorm:"primaryKey;size:36"`
	DigestID         string    `gorm:"size:36;not null"`
	DossierID        string    `gorm:"size:36;not null"`
	SectionName      string    `gorm:"not null"`
	ImportanceBucket string    `gorm:"not null;default:'normal'"`
	Position         int       `gorm:"not null"`
	IsFeatured       bool      `gorm:"not null;default:false"`
	CreatedAt        time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;autoCreateTime"`
}

func (DailyDigestItemModel) TableName() string {
	return "daily_digest_items"
}
