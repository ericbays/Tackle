// Package credentials provides HTTP handlers for credential capture operations.
package credentials

import (
	credsvc "tackle/internal/services/credential"
)

// Deps holds dependencies for credential capture handlers.
type Deps struct {
	Svc *credsvc.Service
}
