package profile

import "errors"

var ErrNotFound = errors.New("profile not found")

const (
	TypeLLM       = "llm"
	TypeMiniflux  = "miniflux"
	TypePrompts   = "prompts"
	TypePublish   = "publish"
	TypeScheduler = "scheduler"
)

type Version struct {
	ID          string
	ProfileType string
	Name        string
	Version     int
	IsActive    bool
	PayloadJSON []byte
}
