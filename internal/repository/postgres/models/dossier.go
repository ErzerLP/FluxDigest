package models

import "time"

type ArticleDossierModel struct {
	ID                       string    `gorm:"primaryKey;size:36"`
	ArticleID                string    `gorm:"size:36;not null;uniqueIndex:idx_article_dossiers_article_version,priority:1;uniqueIndex:idx_article_dossiers_article_active,where:is_active = true"`
	ProcessingID             string    `gorm:"size:36;not null"`
	DigestDate               time.Time `gorm:"type:date;not null;index:idx_article_dossiers_digest_date,priority:1"`
	Version                  int       `gorm:"not null;uniqueIndex:idx_article_dossiers_article_version,priority:2"`
	IsActive                 bool      `gorm:"not null;default:true;index:idx_article_dossiers_digest_date,priority:2"`
	TitleTranslated          string    `gorm:"not null"`
	SummaryPolished          string    `gorm:"not null"`
	CoreSummary              string    `gorm:"not null"`
	KeyPointsJSON            []byte    `gorm:"type:jsonb;not null;default:'[]'"`
	TopicCategory            string    `gorm:"not null"`
	ImportanceScore          float64   `gorm:"not null"`
	RecommendationReason     string    `gorm:"not null;default:''"`
	ReadingValue             string    `gorm:"not null;default:''"`
	PriorityLevel            string    `gorm:"not null;default:'normal'"`
	ContentPolishedMarkdown  string    `gorm:"not null"`
	AnalysisLongformMarkdown string    `gorm:"not null"`
	BackgroundContext        string    `gorm:"not null;default:''"`
	ImpactAnalysis           string    `gorm:"not null;default:''"`
	DebatePointsJSON         []byte    `gorm:"type:jsonb;not null;default:'[]'"`
	TargetAudience           string    `gorm:"not null;default:''"`
	PublishSuggestion        string    `gorm:"not null;default:'draft'"`
	SuggestionReason         string    `gorm:"not null;default:''"`
	SuggestedChannelsJSON    []byte    `gorm:"type:jsonb;not null;default:'[]'"`
	SuggestedTagsJSON        []byte    `gorm:"type:jsonb;not null;default:'[]'"`
	SuggestedCategoriesJSON  []byte    `gorm:"type:jsonb;not null;default:'[]'"`
	TranslationPromptVersion int       `gorm:"not null"`
	AnalysisPromptVersion    int       `gorm:"not null"`
	DossierPromptVersion     int       `gorm:"not null"`
	LLMProfileVersion        int       `gorm:"not null"`
	CreatedAt                time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;autoCreateTime"`
	UpdatedAt                time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

func (ArticleDossierModel) TableName() string {
	return "article_dossiers"
}
