package models

type ProfileVersionModel struct {
	ID          string `gorm:"primaryKey;size:36"`
	ProfileType string `gorm:"not null;index:idx_profile_versions_type_active,priority:1;uniqueIndex:idx_profile_versions_unique,priority:1"`
	Name        string `gorm:"not null;uniqueIndex:idx_profile_versions_unique,priority:2"`
	Version     int    `gorm:"not null;uniqueIndex:idx_profile_versions_unique,priority:3"`
	IsActive    bool   `gorm:"not null;default:false;index:idx_profile_versions_type_active,priority:2"`
	PayloadJSON []byte `gorm:"type:jsonb;not null"`
}

func (ProfileVersionModel) TableName() string {
	return "profile_versions"
}
