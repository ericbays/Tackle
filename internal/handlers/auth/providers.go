package auth

import (
	"net/http"

	"tackle/pkg/response"
)

type providerInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	ButtonLabel string `json:"button_label"`
}

// Providers handles GET /api/v1/auth/providers.
// In Phase 1, only the local provider is returned. External providers are added later.
func (d *Deps) Providers(w http.ResponseWriter, r *http.Request) {
	providers := []providerInfo{
		{
			Type:        "local",
			Name:        "Local Account",
			ButtonLabel: "Sign in with password",
		},
	}
	response.Success(w, providers)
}
