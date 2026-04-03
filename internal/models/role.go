package models

import "time"

// Role represents a Tackle role as stored in the database.
type Role struct {
	ID          string
	Name        string
	Description string
	IsBuiltin   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
