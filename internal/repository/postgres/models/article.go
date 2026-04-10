package models

type SourceArticleModel struct {
	ID              string `gorm:"primaryKey;size:36"`
	MinifluxEntryID int64  `gorm:"uniqueIndex"`
	Title           string
	URL             string
	Fingerprint     string `gorm:"uniqueIndex"`
}

func (SourceArticleModel) TableName() string {
	return "source_articles"
}
