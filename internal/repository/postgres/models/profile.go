package models

type ProfileVersionModel struct {
	ID          string `gorm:"primaryKey;size:36"`
	ProfileType string `gorm:"index"`
	Name        string
	Version     int
	IsActive    bool
	PayloadJSON []byte `gorm:"type:jsonb"`
}

func (ProfileVersionModel) TableName() string {
	return "profile_versions"
}
