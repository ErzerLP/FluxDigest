package models

import "time"

type ArticleProcessingModel struct {
	ID                string    `gorm:"primaryKey;size:36"`
	ArticleID         string    `gorm:"size:36;not null"`
	TitleTranslated   string    `gorm:"not null"`
	SummaryTranslated string    `gorm:"not null"`
	ContentTranslated string    `gorm:"not null"`
	CoreSummary       string    `gorm:"not null"`
	KeyPointsJSON     []byte    `gorm:"type:jsonb;not null"`
	TopicCategory     string    `gorm:"not null"`
	ImportanceScore   float64   `gorm:"not null"`
	CreatedAt         time.Time `gorm:"not null;autoCreateTime"`
}

func (ArticleProcessingModel) TableName() string {
	return "article_processings"
}
