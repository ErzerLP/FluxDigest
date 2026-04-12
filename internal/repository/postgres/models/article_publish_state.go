package models

import "time"

type ArticlePublishStateModel struct {
	ID             string `gorm:"primaryKey;size:36"`
	DossierID      string `gorm:"size:36;not null;uniqueIndex"`
	State          string `gorm:"not null"`
	ApprovedBy     string `gorm:"not null;default:''"`
	DecisionNote   string `gorm:"not null;default:''"`
	PublishChannel string `gorm:"not null;default:''"`
	RemoteID       string `gorm:"not null;default:''"`
	RemoteURL      string `gorm:"not null;default:''"`
	ErrorMessage   string `gorm:"not null;default:''"`
	PublishedAt    *time.Time
	UpdatedAt      time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

func (ArticlePublishStateModel) TableName() string {
	return "article_publish_states"
}
