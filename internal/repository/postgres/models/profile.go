package models

type ProfileVersionModel struct {
	ID          string `gorm:"primaryKey;size:36"`
	ProfileType string `gorm:"not null;index"`
	Name        string `gorm:"not null"`
	Version     int    `gorm:"not null"`
	IsActive    bool   `gorm:"not null;default:false"`
	PayloadJSON []byte `gorm:"type:jsonb;not null"`
}

func (ProfileVersionModel) TableName() string {
	return "profile_versions"
}
