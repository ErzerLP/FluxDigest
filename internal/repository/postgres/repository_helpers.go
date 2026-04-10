package postgres

import "github.com/google/uuid"

func ensureID(id string) string {
	if id == "" {
		return uuid.NewString()
	}
	return id
}
