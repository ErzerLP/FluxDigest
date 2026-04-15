package models

import "time"

type AdminUserModel struct {
	ID                 string `gorm:"primaryKey;size:36"`
	Username           string `gorm:"not null;uniqueIndex"`
	PasswordHash       string `gorm:"not null"`
	MustChangePassword bool   `gorm:"not null;default:true"`
	LastLoginAt        *time.Time
}

func (AdminUserModel) TableName() string {
	return "admin_users"
}
