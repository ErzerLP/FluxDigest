package models

type SourceArticleModel struct {
	ID              string `gorm:"primaryKey;size:36"`
	MinifluxEntryID int64  `gorm:"uniqueIndex;not null"`
	FeedID          int64  `gorm:"not null"`
	FeedTitle       string `gorm:"not null"`
	Title           string `gorm:"not null"`
	Author          string `gorm:"not null"`
	URL             string `gorm:"not null"`
	ContentHTML     string `gorm:"not null"`
	ContentText     string `gorm:"not null"`
	Fingerprint     string `gorm:"uniqueIndex;not null"`
}

func (SourceArticleModel) TableName() string {
	return "source_articles"
}
