package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tackle/internal/middleware"
	notifsvc "tackle/internal/services/notification"
	"tackle/pkg/response"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// List handles GET /api/v1/notifications — returns the caller's notifications with cursor pagination.
func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	q := r.URL.Query()

	limit := defaultPageSize
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	cursor := q.Get("cursor")

	var clauses []string
	args := []any{}
	nextArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	// Always scope to the authenticated user.
	clauses = append(clauses, "user_id = "+nextArg(claims.Subject)+"::uuid")

	if v := q.Get("category"); v != "" {
		clauses = append(clauses, "category = "+nextArg(v))
	}
	if v := q.Get("severity"); v != "" {
		clauses = append(clauses, "severity = "+nextArg(v))
	}
	if v := q.Get("is_read"); v != "" {
		switch strings.ToLower(v) {
		case "true":
			clauses = append(clauses, "is_read = TRUE")
		case "false":
			clauses = append(clauses, "is_read = FALSE")
		}
	}
	if v := q.Get("created_after"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			clauses = append(clauses, "created_at >= "+nextArg(t))
		}
	}
	if v := q.Get("created_before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			clauses = append(clauses, "created_at <= "+nextArg(t))
		}
	}
	if cursor != "" {
		// Cursor format: "timestamp:id"
		parts := strings.SplitN(cursor, ":", 2)
		if len(parts) == 2 {
			if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				clauses = append(clauses,
					"(created_at < "+nextArg(ts)+
						" OR (created_at = "+nextArg(ts)+
						" AND id::text < "+nextArg(parts[1])+"))")
			}
		}
	}

	where := " WHERE " + strings.Join(clauses, " AND ")
	query := `SELECT id, user_id, category, severity, title, body, resource_type, resource_id,
		action_url, is_read, expires_at, created_at
		FROM notifications` + where + ` ORDER BY created_at DESC, id DESC LIMIT ` + nextArg(limit+1)

	rows, err := d.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query notifications", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	items := make([]notifsvc.Notification, 0, limit)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read notification", http.StatusInternalServerError, correlationID)
			return
		}
		items = append(items, *n)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate notifications", http.StatusInternalServerError, correlationID)
		return
	}

	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		c := last.CreatedAt.UTC().Format(time.RFC3339Nano) + ":" + last.ID
		nextCursor = &c
	}

	type listResp struct {
		Data       []notifsvc.Notification `json:"data"`
		NextCursor *string                 `json:"next_cursor"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResp{Data: items, NextCursor: nextCursor})
}

// scanNotification scans one row from the notifications table.
func scanNotification(rows *sql.Rows) (*notifsvc.Notification, error) {
	var n notifsvc.Notification
	var resourceType, resourceID, actionURL sql.NullString
	var expiresAt sql.NullTime

	err := rows.Scan(
		&n.ID, &n.UserID, &n.Category, &n.Severity, &n.Title, &n.Body,
		&resourceType, &resourceID, &actionURL, &n.IsRead, &expiresAt, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if resourceType.Valid {
		s := resourceType.String
		n.ResourceType = &s
	}
	if resourceID.Valid {
		s := resourceID.String
		n.ResourceID = &s
	}
	if actionURL.Valid {
		s := actionURL.String
		n.ActionURL = &s
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		n.ExpiresAt = &t
	}
	return &n, nil
}
