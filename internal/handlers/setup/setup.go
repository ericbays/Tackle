package setup

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"tackle/internal/middleware"
	"tackle/internal/models"
	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/internal/services/rbac"
	"tackle/pkg/response"
)

var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

type setupRequest struct {
	Username             string `json:"username"`
	Email                string `json:"email"`
	DisplayName          string `json:"display_name"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type setupResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	User         userSummary `json:"user"`
}

type userSummary struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// Setup handles POST /api/v1/setup.
// Creates the initial admin account, assigns the admin role, and auto-logs in.
// Protected by RequireSetupPending middleware — only callable when no users exist.
func (d *Deps) Setup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Validate inputs.
	fieldErrors := validateSetupRequest(req, d.Policy)
	if len(fieldErrors) > 0 {
		writeValidationError(w, fieldErrors, correlationID)
		return
	}

	// Hash password.
	hash, err := authsvc.HashPassword(req.Password)
	if err != nil {
		slog.Error("setup: hash password", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to process request", http.StatusInternalServerError, correlationID)
		return
	}

	// Begin transaction.
	tx, err := d.DB.BeginTx(r.Context(), nil)
	if err != nil {
		slog.Error("setup: begin tx", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to process request", http.StatusInternalServerError, correlationID)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	// Insert user.
	var userID string
	const insertUser = `
		INSERT INTO users (username, email, display_name, password_hash, is_initial_admin, auth_provider)
		VALUES ($1, $2, $3, $4, TRUE, 'local')
		RETURNING id`
	err = tx.QueryRowContext(r.Context(), insertUser,
		req.Username, strings.ToLower(req.Email), req.DisplayName, hash,
	).Scan(&userID)
	if err != nil {
		slog.Error("setup: insert user", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to create user", http.StatusInternalServerError, correlationID)
		return
	}

	// Look up admin role ID.
	var roleID string
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM roles WHERE name = $1`, rbac.RoleAdmin).Scan(&roleID)
	if err != nil {
		slog.Error("setup: look up admin role", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to find admin role", http.StatusInternalServerError, correlationID)
		return
	}

	// Assign admin role.
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
		userID, roleID,
	)
	if err != nil {
		slog.Error("setup: assign admin role", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to assign role", http.StatusInternalServerError, correlationID)
		return
	}

	// Build permission list for admin (all permissions).
	allPerms := rbac.All()
	permStrings := make([]string, len(allPerms))
	for i, p := range allPerms {
		permStrings[i] = string(p)
	}

	// Commit transaction before issuing tokens — sessions FK references users.
	if err := tx.Commit(); err != nil {
		slog.Error("setup: commit tx", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to complete setup", http.StatusInternalServerError, correlationID)
		return
	}

	// Issue JWT.
	user := &models.User{
		ID:          userID,
		Username:    req.Username,
		Email:       strings.ToLower(req.Email),
		DisplayName: req.DisplayName,
	}
	accessToken, err := d.JWTSvc.Issue(user, rbac.RoleAdmin, permStrings)
	if err != nil {
		slog.Error("setup: issue jwt", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to issue token", http.StatusInternalServerError, correlationID)
		return
	}

	// Issue refresh token — user is now committed so FK is satisfied.
	accessHash := authsvc.HashTokenPublic(accessToken)
	rawRefresh, err := d.RefreshSvc.Issue(r.Context(), userID, accessHash, r.RemoteAddr, r.UserAgent(), 7*24*time.Hour)
	if err != nil {
		slog.Error("setup: issue refresh token", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to issue session", http.StatusInternalServerError, correlationID)
		return
	}

	slog.Info("system.setup.complete",
		"user_id", userID,
		"username", req.Username,
		"correlation_id", correlationID,
	)
	if d.AuditSvc != nil {
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategorySystem,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &userID,
			ActorLabel:    req.Username,
			Action:        "system.setup.complete",
			CorrelationID: correlationID,
			Details:       map[string]any{"username": req.Username},
		})
	}

	// Set refresh token as HTTP-only cookie for browser clients.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rawRefresh,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	response.Created(w, setupResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    900,
		User: userSummary{
			ID:          userID,
			Username:    req.Username,
			DisplayName: req.DisplayName,
			Roles:       []string{rbac.RoleAdmin},
			Permissions: permStrings,
		},
	})
}

// validationError describes a single field validation failure.
type validationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type validationErrorEnvelope struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  []validationError `json:"fields"`
	} `json:"error"`
}

func writeValidationError(w http.ResponseWriter, fields []validationError, correlationID string) {
	env := validationErrorEnvelope{}
	env.Error.Code = "VALIDATION_ERROR"
	env.Error.Message = "request validation failed"
	env.Error.Fields = fields
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(env)
}

func validateSetupRequest(req setupRequest, policy authsvc.PasswordPolicy) []validationError {
	var errs []validationError

	if !usernameRE.MatchString(req.Username) {
		errs = append(errs, validationError{
			Field:   "username",
			Message: "username must be 3–64 characters and contain only letters, digits, underscores, or hyphens",
		})
	}

	if !isValidEmail(req.Email) {
		errs = append(errs, validationError{
			Field:   "email",
			Message: "email must be a valid address",
		})
	}

	if strings.TrimSpace(req.DisplayName) == "" || len(req.DisplayName) > 128 {
		errs = append(errs, validationError{
			Field:   "display_name",
			Message: "display_name must be non-empty and at most 128 characters",
		})
	}

	if err := policy.Validate(req.Password); err != nil {
		errs = append(errs, validationError{
			Field:   "password",
			Message: err.Error(),
		})
	}

	if req.Password != req.PasswordConfirmation {
		errs = append(errs, validationError{
			Field:   "password_confirmation",
			Message: "password confirmation does not match",
		})
	}

	return errs
}

// isValidEmail performs a basic RFC 5322-style email check.
func isValidEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) == 0 || len(domain) < 3 {
		return false
	}
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
}
