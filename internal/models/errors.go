package models

// ErrAccountLocked is returned when a locked account attempts to authenticate.
type ErrAccountLocked struct{}

func (e *ErrAccountLocked) Error() string { return "account locked" }

// ErrInvalidCredentials is returned when authentication fails due to bad credentials.
type ErrInvalidCredentials struct{}

func (e *ErrInvalidCredentials) Error() string { return "invalid credentials" }
