package auth

import (
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type meResponse struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Provider    string   `json:"provider"`
}

// Me handles GET /api/v1/auth/me.
func (d *Deps) Me(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	ur, err := findUserByID(r.Context(), d.DB, claims.Subject)
	if err != nil {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	}

	response.Success(w, meResponse{
		ID:          ur.user.ID,
		Username:    ur.user.Username,
		Email:       ur.user.Email,
		DisplayName: ur.user.DisplayName,
		Role:        ur.roleName,
		Permissions: ur.permissions,
		Provider:    ur.user.AuthProvider,
	})
}
