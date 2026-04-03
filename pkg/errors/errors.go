// Package errors defines structured error types for the Tackle API.
package errors

import "fmt"

// AppError is a structured application error carrying an HTTP status code and error code.
type AppError struct {
	Code    string
	Message string
	Status  int
	Err     error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error, supporting errors.Is and errors.As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError.
func New(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, Status: status}
}

// Wrap creates a new AppError wrapping an existing error.
func Wrap(code, message string, status int, err error) *AppError {
	return &AppError{Code: code, Message: message, Status: status, Err: err}
}
