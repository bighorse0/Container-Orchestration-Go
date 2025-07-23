package api

import (
	"github.com/google/uuid"
)

// generateUID generates a unique identifier
func generateUID() string {
	return uuid.New().String()
}