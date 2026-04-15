package admin

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("admin user not found")

type User struct {
	ID                 string
	Username           string
	PasswordHash       string
	MustChangePassword bool
	LastLoginAt        time.Time
}
