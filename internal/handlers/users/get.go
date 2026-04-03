package users

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

type userDetailResponse struct {
	ID                  string     `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email"`
	DisplayName         string     `json:"display_name"`
	Status              string     `json:"status"`
	RoleID              *string    `json:"role_id"`
	RoleName            *string    `json:"role_name"`
	Permissions         []string   `json:"permissions"`
	IsInitialAdmin      bool       `json:"is_initial_admin"`
	ForcePasswordChange bool       `json:"force_password_change"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// GetUser handles GET /api/v1/users/{id}.
func (d *Deps) GetUser(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	callerClaims := middleware.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var u userDetailResponse
	err := d.DB.QueryRowContext(r.Context(), `
		SELECT u.id, u.username, u.email, u.display_name, u.status,
		       ur.role_id, r.name,
		       u.is_initial_admin, u.force_password_change,
		       u.created_at, u.updated_at
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.id = $1`, id,
	).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Status,
		&u.RoleID, &u.RoleName,
		&u.IsInitialAdmin, &u.ForcePasswordChange,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	}
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch user", http.StatusInternalServerError, correlationID)
		return
	}

	// Fetch permissions for this user's role.
	u.Permissions = []string{}
	if u.RoleID != nil {
		rows, err := d.DB.QueryContext(r.Context(), `
			SELECT p.name
			FROM permissions p
			JOIN role_permissions rp ON rp.permission_id = p.id
			WHERE rp.role_id = $1
			ORDER BY p.name`, *u.RoleID)
		if err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to fetch permissions", http.StatusInternalServerError, correlationID)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var perm string
			if err := rows.Scan(&perm); err != nil {
				response.Error(w, "INTERNAL_ERROR", "failed to scan permission", http.StatusInternalServerError, correlationID)
				return
			}
			u.Permissions = append(u.Permissions, perm)
		}
		if err := rows.Err(); err != nil {
			response.Error(w, "INTERNAL_ERROR", "permission query error", http.StatusInternalServerError, correlationID)
			return
		}
	}

	// Audit sensitive PII read (REQ-LOG-005).
	if d.AuditSvc != nil && callerClaims != nil {
		actorID := callerClaims.Subject
		resType := "user"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "user.detail.viewed",
			ResourceType:  &resType,
			ResourceID:    &u.ID,
			CorrelationID: correlationID,
		})
	}

	response.Success(w, u)
}
