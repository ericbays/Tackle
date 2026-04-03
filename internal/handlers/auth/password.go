package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/pkg/response"
)

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// ChangePassword handles PUT /api/v1/users/me/password.
func (d *Deps) ChangePassword(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	ur, err := findUserByID(r.Context(), d.DB, claims.Subject)
	if err != nil {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	}

	if ur.user.PasswordHash == nil {
		response.Error(w, "BAD_REQUEST", "no local password set", http.StatusBadRequest, correlationID)
		return
	}
	if err := authsvc.ComparePassword(*ur.user.PasswordHash, req.CurrentPassword); err != nil {
		response.Error(w, "UNAUTHORIZED", "current password is incorrect", http.StatusUnauthorized, correlationID)
		return
	}
	if err := d.Policy.Validate(req.NewPassword); err != nil {
		response.Error(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	if authsvc.IsBreached(req.NewPassword) {
		response.Error(w, "VALIDATION_ERROR", "password appears in a list of commonly breached passwords", http.StatusBadRequest, correlationID)
		return
	}

	reused, err := d.HistoryChecker.IsReused(r.Context(), ur.user.ID, req.NewPassword, 5)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "password history check failed", http.StatusInternalServerError, correlationID)
		return
	}
	if reused {
		response.Error(w, "VALIDATION_ERROR", "password has been used recently", http.StatusBadRequest, correlationID)
		return
	}

	newHash, err := authsvc.HashPassword(req.NewPassword)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to hash password", http.StatusInternalServerError, correlationID)
		return
	}

	if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, ur.user.ID); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to update password", http.StatusInternalServerError, correlationID)
		return
	}
	if err := d.HistoryChecker.Record(r.Context(), ur.user.ID, newHash); err != nil {
		slog.Warn("failed to record password history", "user_id", ur.user.ID, "err", err)
	}

	// Revoke all other sessions for this user.
	if err := d.RefreshSvc.RevokeAll(r.Context(), ur.user.ID); err != nil {
		slog.Warn("failed to revoke sessions after password change", "user_id", ur.user.ID, "err", err)
	}

	slog.Info("password.changed", "user_id", ur.user.ID, "correlation_id", correlationID)
	if d.AuditSvc != nil {
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &ur.user.ID,
			ActorLabel:    ur.user.Username,
			Action:        "auth.password.change",
			CorrelationID: correlationID,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

// ResetPassword handles PUT /api/v1/users/{id}/password (admin only).
func (d *Deps) ResetPassword(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	targetID := chi.URLParam(r, "id")
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Policy.Validate(req.NewPassword); err != nil {
		response.Error(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	if authsvc.IsBreached(req.NewPassword) {
		response.Error(w, "VALIDATION_ERROR", "password appears in a list of commonly breached passwords", http.StatusBadRequest, correlationID)
		return
	}

	newHash, err := authsvc.HashPassword(req.NewPassword)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to hash password", http.StatusInternalServerError, correlationID)
		return
	}

	result, err := d.DB.ExecContext(r.Context(),
		`UPDATE users SET password_hash = $1, force_password_change = TRUE WHERE id = $2`,
		newHash, targetID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to update password", http.StatusInternalServerError, correlationID)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	}

	if err := d.HistoryChecker.Record(r.Context(), targetID, newHash); err != nil {
		slog.Warn("failed to record password history", "user_id", targetID, "err", err)
	}

	// Revoke all sessions for the target user after admin password reset.
	if err := d.RefreshSvc.RevokeAll(r.Context(), targetID); err != nil {
		slog.Warn("failed to revoke sessions after password reset", "target_user_id", targetID, "err", err)
	}

	slog.Info("password.reset", "actor_id", claims.Subject, "target_user_id", targetID, "correlation_id", correlationID)
	if d.AuditSvc != nil {
		actorID := claims.Subject
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    claims.Username,
			Action:        "auth.password.reset",
			CorrelationID: correlationID,
			Details:       map[string]any{"target_user_id": targetID},
		})
	}
	w.WriteHeader(http.StatusNoContent)
}
