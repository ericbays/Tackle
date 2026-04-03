package users

import (
	"encoding/json"
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Email       *string `json:"email"`
}

// UpdateOwnProfile handles PUT /api/v1/users/me/profile — update own display_name and email.
func (d *Deps) UpdateOwnProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if req.Email != nil && !validEmail.MatchString(*req.Email) {
		response.Error(w, "VALIDATION_ERROR", "valid email is required", http.StatusBadRequest, correlationID)
		return
	}

	if req.DisplayName != nil {
		if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`, *req.DisplayName, callerClaims.Subject); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update display_name", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.Email != nil {
		if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET email = $1, updated_at = now() WHERE id = $2`, *req.Email, callerClaims.Subject); err != nil {
			if isUniqueViolation(err) {
				response.Error(w, "CONFLICT", "email already in use", http.StatusConflict, correlationID)
				return
			}
			response.Error(w, "INTERNAL_ERROR", "failed to update email", http.StatusInternalServerError, correlationID)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
