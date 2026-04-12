package models

import "time"

type ArticleProcessingModel struct {
	ID                       string    `gorm:"primaryKey;size:36"`
	ArticleID                string    `gorm:"size:36;not null"`
	TitleTranslated          string    `gorm:"not null"`
	SummaryTranslated        string    `gorm:"not null"`
	ContentTranslated        string    `gorm:"not null"`
	CoreSummary              string    `gorm:"not null"`
	KeyPointsJSON            []byte    `gorm:"type:jsonb;not null"`
	TopicCategory            string    `gorm:"not null"`
	ImportanceScore          float64   `gorm:"not null"`
	TranslationPromptVersion int       `gorm:"not null;default:1"`
	AnalysisPromptVersion    int       `gorm:"not null;default:1"`
	LLMProfileVersion        int       `gorm:"not null;default:1"`
	Status                   string    `gorm:"not null;default:'completed'"`
	ErrorMessage             string    `gorm:"not null;default:''"`
	ProcessedAt              time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	CreatedAt                time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;autoCreateTime"`
}

func (ArticleProcessingModel) TableName() string {
	return "article_processings"
}
