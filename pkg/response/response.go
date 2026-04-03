// Package response provides standardized JSON API response helpers.
package response

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Pagination describes a paginated list result.
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// CursorPagination describes a cursor-paginated list result.
type CursorPagination struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type cursorListEnvelope struct {
	Data       any              `json:"data"`
	Pagination CursorPagination `json:"pagination"`
}

// CursorToken holds the fields encoded in a pagination cursor.
type CursorToken struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// EncodeCursor serialises a cursor token to a URL-safe base64 string.
func EncodeCursor(id string, createdAt time.Time) string {
	b, _ := json.Marshal(CursorToken{ID: id, CreatedAt: createdAt})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses a cursor string back into its components.
func DecodeCursor(cursor string) (CursorToken, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return CursorToken{}, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var tok CursorToken
	if err := json.Unmarshal(b, &tok); err != nil {
		return CursorToken{}, fmt.Errorf("invalid cursor payload: %w", err)
	}
	return tok, nil
}

// CursorList writes a 200 OK response with cursor-paginated data.
func CursorList(w http.ResponseWriter, data any, pagination CursorPagination) {
	writeJSON(w, http.StatusOK, cursorListEnvelope{Data: data, Pagination: pagination})
}

type successEnvelope struct {
	Data any `json:"data"`
}

type listEnvelope struct {
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type errorDetail struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

// Success writes a 200 OK response with the given data wrapped in {"data": ...}.
func Success(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, successEnvelope{Data: data})
}

// Created writes a 201 Created response with the given data wrapped in {"data": ...}.
func Created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, successEnvelope{Data: data})
}

// Accepted writes a 202 Accepted response with the given data wrapped in {"data": ...}.
func Accepted(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusAccepted, successEnvelope{Data: data})
}

// List writes a 200 OK response with paginated data.
func List(w http.ResponseWriter, data any, pagination Pagination) {
	writeJSON(w, http.StatusOK, listEnvelope{Data: data, Pagination: pagination})
}

// FieldError describes a validation failure on a single field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"` // e.g. "required", "invalid_format", "too_long", "duplicate"
}

type validationEnvelope struct {
	Error   errorDetail  `json:"error"`
	Details []FieldError `json:"details"`
}

// ValidationFailed writes a 422 Unprocessable Entity response with per-field error details.
func ValidationFailed(w http.ResponseWriter, fields []FieldError, correlationID string) {
	writeJSON(w, http.StatusUnprocessableEntity, validationEnvelope{
		Error: errorDetail{
			Code:          "VALIDATION_FAILED",
			Message:       "one or more fields failed validation",
			CorrelationID: correlationID,
		},
		Details: fields,
	})
}

// Error writes an error response with the given HTTP status code.
func Error(w http.ResponseWriter, code, message string, status int, correlationID string) {
	writeJSON(w, status, errorEnvelope{
		Error: errorDetail{
			Code:          code,
			Message:       message,
			CorrelationID: correlationID,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
