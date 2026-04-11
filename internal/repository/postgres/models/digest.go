package models

import "time"

type DailyDigestModel struct {
	ID              string    `gorm:"primaryKey;size:36"`
	DigestDate      time.Time `gorm:"type:date;not null;unique"`
	Title           string    `gorm:"not null"`
	Subtitle        string    `gorm:"not null"`
	ContentMarkdown string    `gorm:"not null"`
	ContentHTML     string    `gorm:"not null"`
	RemoteURL       string    `gorm:"not null;default:''"`
	CreatedAt       time.Time `gorm:"not null;autoCreateTime"`
}

func (DailyDigestModel) TableName() string {
	return "daily_digests"
}
