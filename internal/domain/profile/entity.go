package profile

import "errors"

var ErrNotFound = errors.New("profile not found")

type Version struct {
	ID          string
	ProfileType string
	Name        string
	Version     int
	IsActive    bool
	PayloadJSON []byte
}
