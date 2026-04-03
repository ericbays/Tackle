// Package models defines shared data types used across the Tackle platform.
package models

import "time"

// User represents a Tackle user account as stored in the database.
type User struct {
	ID                  string
	Email               string
	Username            string
	PasswordHash        *string
	DisplayName         string
	IsInitialAdmin      bool
	AuthProvider        string
	ExternalID          *string
	Status              string
	ForcePasswordChange bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
