package users

import (
	"fmt"
	"net/http"
	"time"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type userListItem struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	DisplayName        string     `json:"display_name"`
	Status             string     `json:"status"`
	RoleID             *string    `json:"role_id"`
	RoleName           *string    `json:"role_name"`
	IsInitialAdmin     bool       `json:"is_initial_admin"`
	ForcePasswordChange bool      `json:"force_password_change"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLoginAt        *time.Time `json:"last_login_at"`
}

type userListResponse struct {
	Data       []userListItem `json:"data"`
	Total      int            `json:"total"`
	NextCursor *string        `json:"next_cursor,omitempty"`
}

// ListUsers handles GET /api/v1/users.
// Supports cursor-based pagination, filtering by status/role/search, and sorting.
func (d *Deps) ListUsers(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	q := r.URL.Query()
	status := q.Get("status")
	roleFilter := q.Get("role")
	search := q.Get("search")
	sortBy := q.Get("sort")
	cursor := q.Get("cursor")
	limitStr := q.Get("limit")

	limit := 25
	if limitStr == "10" {
		limit = 10
	} else if limitStr == "50" {
		limit = 50
	}

	allowedSorts := map[string]string{
		"username":      "u.username",
		"display_name":  "u.display_name",
		"created_at":    "u.created_at",
		"last_login_at": "u.created_at", // last_login_at not in schema; fall back to created_at
	}
	orderCol := "u.username"
	if col, ok := allowedSorts[sortBy]; ok {
		orderCol = col
	}

	// Build WHERE clauses dynamically.
	where := "WHERE 1=1"
	args := []any{}
	idx := 1

	if status != "" {
		where += fmt.Sprintf(" AND u.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if roleFilter != "" {
		where += fmt.Sprintf(" AND r.name = $%d", idx)
		args = append(args, roleFilter)
		idx++
	}
	if search != "" {
		like := "%" + search + "%"
		where += fmt.Sprintf(" AND (u.username ILIKE $%d OR u.display_name ILIKE $%d OR u.email ILIKE $%d)", idx, idx+1, idx+2)
		args = append(args, like, like, like)
		idx += 3
	}
	if cursor != "" {
		where += fmt.Sprintf(" AND %s > $%d", orderCol, idx)
		args = append(args, cursor)
		idx++
	}

	// Count total.
	countQ := `SELECT COUNT(*) FROM users u LEFT JOIN user_roles ur ON ur.user_id = u.id LEFT JOIN roles r ON r.id = ur.role_id ` + where
	var total int
	if err := d.DB.QueryRowContext(r.Context(), countQ, args...).Scan(&total); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to count users", http.StatusInternalServerError, correlationID)
		return
	}

	fetchQ := fmt.Sprintf(`
		SELECT u.id, u.username, u.email, u.display_name, u.status,
		       ur.role_id, r.name, u.is_initial_admin, u.force_password_change,
		       u.created_at
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		%s
		ORDER BY %s ASC
		LIMIT $%d`, where, orderCol, idx)
	args = append(args, limit+1)

	rows, err := d.DB.QueryContext(r.Context(), fetchQ, args...)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list users", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var items []userListItem
	for rows.Next() {
		var u userListItem
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Status,
			&u.RoleID, &u.RoleName, &u.IsInitialAdmin, &u.ForcePasswordChange,
			&u.CreatedAt,
		); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan user", http.StatusInternalServerError, correlationID)
			return
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "user query error", http.StatusInternalServerError, correlationID)
		return
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nc := last.Username
		nextCursor = &nc
	}

	if items == nil {
		items = []userListItem{}
	}

	response.Success(w, userListResponse{Data: items, Total: total, NextCursor: nextCursor})
}
