package users

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

type updateUserRequest struct {
	DisplayName *string `json:"display_name"`
	Email       *string `json:"email"`
	Status      *string `json:"status"`
}

// UpdateUser handles PUT /api/v1/users/{id}.
func (d *Deps) UpdateUser(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	targetID := chi.URLParam(r, "id")
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Validate fields.
	var fieldErrors []response.FieldError
	if req.Status != nil {
		switch *req.Status {
		case "active", "inactive", "locked":
		default:
			fieldErrors = append(fieldErrors, response.FieldError{Field: "status", Message: "status must be active, inactive, or locked", Code: "invalid_value"})
		}
	}
	if req.Email != nil && !validEmail.MatchString(*req.Email) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "email", Message: "invalid email format", Code: "invalid_format"})
	}
	if len(fieldErrors) > 0 {
		response.ValidationFailed(w, fieldErrors, correlationID)
		return
	}

	// Check initial admin guard.
	var isInitialAdmin bool
	if err := d.DB.QueryRowContext(r.Context(), `SELECT is_initial_admin FROM users WHERE id = $1`, targetID).Scan(&isInitialAdmin); err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	} else if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch user", http.StatusInternalServerError, correlationID)
		return
	}

	if isInitialAdmin && req.Status != nil {
		response.Error(w, "CONFLICT", "cannot modify initial admin", http.StatusConflict, correlationID)
		return
	}

	// Apply updates.
	if req.DisplayName != nil {
		if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`, *req.DisplayName, targetID); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update display_name", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.Email != nil {
		if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET email = $1, updated_at = now() WHERE id = $2`, *req.Email, targetID); err != nil {
			if isUniqueViolation(err) {
				response.Error(w, "CONFLICT", "email already in use", http.StatusConflict, correlationID)
				return
			}
			response.Error(w, "INTERNAL_ERROR", "failed to update email", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.Status != nil {
		if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, targetID); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update status", http.StatusInternalServerError, correlationID)
			return
		}

		// On account lock, revoke all active sessions immediately.
		if *req.Status == "locked" && d.RefreshSvc != nil {
			_ = d.RefreshSvc.RevokeAll(r.Context(), targetID)
		}
	}

	// Determine the appropriate audit action based on what changed.
	auditAction := "user.updated"
	if req.Status != nil {
		switch *req.Status {
		case "locked":
			auditAction = "auth.account.locked"
		case "active":
			auditAction = "auth.account.unlocked"
		}
	}

	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "user"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        auditAction,
			ResourceType:  &resType,
			ResourceID:    &targetID,
			CorrelationID: correlationID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
