package users

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/pkg/response"
)

type createUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	RoleID      string `json:"role_id"`
}

type createUserResponse struct {
	ID                  string    `json:"id"`
	Username            string    `json:"username"`
	Email               string    `json:"email"`
	DisplayName         string    `json:"display_name"`
	Status              string    `json:"status"`
	RoleID              string    `json:"role_id"`
	RoleName            string    `json:"role_name"`
	IsInitialAdmin      bool      `json:"is_initial_admin"`
	ForcePasswordChange bool      `json:"force_password_change"`
	CreatedAt           time.Time `json:"created_at"`
}

var validEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// CreateUser handles POST /api/v1/users.
func (d *Deps) CreateUser(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Validate required fields — collect all errors.
	var fieldErrors []response.FieldError
	if req.Username == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "username", Message: "username is required", Code: "required"})
	}
	if req.Email == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "email", Message: "email is required", Code: "required"})
	} else if !validEmail.MatchString(req.Email) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "email", Message: "invalid email format", Code: "invalid_format"})
	}
	if req.DisplayName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "display_name", Message: "display_name is required", Code: "required"})
	}
	if len(req.Password) < 12 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "password", Message: "password must be at least 12 characters", Code: "too_short"})
	}
	if req.RoleID == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "role_id", Message: "role_id is required", Code: "required"})
	}
	if len(fieldErrors) > 0 {
		response.ValidationFailed(w, fieldErrors, correlationID)
		return
	}

	// Validate role exists.
	var roleName string
	if err := d.DB.QueryRowContext(r.Context(), `SELECT name FROM roles WHERE id = $1`, req.RoleID).Scan(&roleName); err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "role not found", http.StatusNotFound, correlationID)
		return
	} else if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to verify role", http.StatusInternalServerError, correlationID)
		return
	}

	// Hash password (bcrypt cost 12).
	hash, err := authsvc.HashPassword(req.Password)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to hash password", http.StatusInternalServerError, correlationID)
		return
	}

	tx, err := d.DB.BeginTx(r.Context(), nil)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to start transaction", http.StatusInternalServerError, correlationID)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var newUser createUserResponse
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO users (username, email, display_name, password_hash, status, force_password_change)
		VALUES ($1, $2, $3, $4, 'active', TRUE)
		RETURNING id, username, email, display_name, status, is_initial_admin, force_password_change, created_at`,
		req.Username, req.Email, req.DisplayName, hash,
	).Scan(
		&newUser.ID, &newUser.Username, &newUser.Email, &newUser.DisplayName,
		&newUser.Status, &newUser.IsInitialAdmin, &newUser.ForcePasswordChange, &newUser.CreatedAt,
	)
	if err != nil {
		// Unique constraint violation.
		if isUniqueViolation(err) {
			response.Error(w, "CONFLICT", "username or email already exists", http.StatusConflict, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to create user", http.StatusInternalServerError, correlationID)
		return
	}

	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, newUser.ID, req.RoleID); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to assign role", http.StatusInternalServerError, correlationID)
		return
	}

	if err := tx.Commit(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to commit", http.StatusInternalServerError, correlationID)
		return
	}

	newUser.RoleID = req.RoleID
	newUser.RoleName = roleName

	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "user"
		resID := newUser.ID
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "user.created",
			ResourceType:  &resType,
			ResourceID:    &resID,
			CorrelationID: correlationID,
			Details:       map[string]any{"username": newUser.Username, "role": roleName},
		})
	}

	w.WriteHeader(http.StatusCreated)
	response.Success(w, newUser)
}

// isUniqueViolation returns true if err is a PostgreSQL unique-constraint error.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pq error code 23505 = unique_violation; check string to avoid importing pq.
	s := err.Error()
	return len(s) > 5 && (s[len(s)-5:] == "23505" || contains(s, "unique") || contains(s, "duplicate"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
