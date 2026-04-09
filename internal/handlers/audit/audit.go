package audit

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tackle/internal/middleware"
	auditsvc "tackle/internal/services/audit"
	"tackle/pkg/response"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// logEntryResponse is the JSON representation of an audit log entry.
type logEntryResponse struct {
	ID            string         `json:"id"`
	Timestamp     time.Time      `json:"timestamp"`
	Category      string         `json:"category"`
	Severity      string         `json:"severity"`
	ActorType     string         `json:"actor_type"`
	ActorID       *string        `json:"actor_id"`
	ActorLabel    string         `json:"actor_label"`
	Action        string         `json:"action"`
	ResourceType  *string        `json:"resource_type"`
	ResourceID    *string        `json:"resource_id"`
	Details       map[string]any `json:"details"`
	CorrelationID string         `json:"correlation_id"`
	SourceIP      *string        `json:"source_ip"`
	SessionID     *string        `json:"session_id"`
	CampaignID       *string        `json:"campaign_id"`
	Checksum         string         `json:"checksum"`
	PreviousChecksum string         `json:"previous_checksum,omitempty"`
}

// List handles GET /api/v1/logs/audit — returns paginated, filtered audit log entries.
// Operators are scoped to their own actor_id; Admins and Engineers see all entries.
func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	q := r.URL.Query()

	// Pagination.
	limit := defaultLimit
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	cursor := q.Get("cursor") // last seen ID from previous page

	// Build WHERE clauses and args.
	var clauses []string
	args := []any{}
	nextArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	// Operators see only their own entries.
	if claims.Role == "operator" {
		clauses = append(clauses, "actor_id = "+nextArg(claims.Subject)+"::uuid")
	} else if actorID := q.Get("actor_id"); actorID != "" {
		clauses = append(clauses, "actor_id = "+nextArg(actorID)+"::uuid")
	}

	if v := q.Get("category"); v != "" {
		clauses = append(clauses, "category = "+nextArg(v))
	}
	if v := q.Get("severity"); v != "" {
		clauses = append(clauses, "severity = "+nextArg(v))
	}
	if v := q.Get("campaign_id"); v != "" {
		clauses = append(clauses, "campaign_id = "+nextArg(v)+"::uuid")
	}
	if v := q.Get("correlation_id"); v != "" {
		clauses = append(clauses, "correlation_id = "+nextArg(v)+"::uuid")
	}
	if v := q.Get("action"); v != "" {
		// Support prefix matching with trailing *.
		if strings.HasSuffix(v, "*") {
			prefix := strings.TrimSuffix(v, "*")
			clauses = append(clauses, "action LIKE "+nextArg(prefix+"%"))
		} else {
			clauses = append(clauses, "action = "+nextArg(v))
		}
	}
	if v := q.Get("resource_type"); v != "" {
		clauses = append(clauses, "resource_type = "+nextArg(v))
	}
	if v := q.Get("resource_id"); v != "" {
		clauses = append(clauses, "resource_id = "+nextArg(v)+"::uuid")
	}
	if v := q.Get("created_after"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			clauses = append(clauses, "timestamp >= "+nextArg(t))
		}
	}
	if v := q.Get("created_before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			clauses = append(clauses, "timestamp <= "+nextArg(t))
		}
	}
	if v := q.Get("search"); v != "" {
		searchArg := nextArg("%" + v + "%")
		clauses = append(clauses, "(action ILIKE "+searchArg+
			" OR details::text ILIKE "+searchArg+
			" OR actor_label ILIKE "+searchArg+
			" OR severity ILIKE "+searchArg+
			" OR source_ip::text ILIKE "+searchArg+
			" OR resource_type ILIKE "+searchArg+")")
	}
	if cursor != "" {
		// Cursor is (timestamp, id) encoded as "timestamp:id" for stable pagination.
		parts := strings.SplitN(cursor, ":", 2)
		if len(parts) == 2 {
			if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				clauses = append(clauses, "(timestamp < "+nextArg(ts)+" OR (timestamp = "+nextArg(ts)+" AND id::text < "+nextArg(parts[1])+"))")
			}
		}
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	query := `SELECT id, timestamp, category, severity, actor_type, actor_id, actor_label, action,
		resource_type, resource_id, details, correlation_id, source_ip, session_id, campaign_id, checksum,
		previous_checksum
		FROM audit_logs` + where + ` ORDER BY timestamp DESC, id DESC LIMIT ` + nextArg(limit+1)

	rows, err := d.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query audit logs", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	entries := make([]logEntryResponse, 0, limit)
	for rows.Next() {
		e, err := scanRow(rows)
		if err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read audit log", http.StatusInternalServerError, correlationID)
			return
		}
		entries = append(entries, *e)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate audit logs", http.StatusInternalServerError, correlationID)
		return
	}

	// Determine next cursor.
	var nextCursor *string
	if len(entries) > limit {
		entries = entries[:limit]
		last := entries[len(entries)-1]
		c := last.Timestamp.UTC().Format(time.RFC3339Nano) + ":" + last.ID
		nextCursor = &c
	}

	type listResp struct {
		Data       []logEntryResponse `json:"data"`
		NextCursor *string            `json:"next_cursor"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResp{Data: entries, NextCursor: nextCursor})
}

// scanRow scans a single audit_logs row into a logEntryResponse.
func scanRow(rows *sql.Rows) (*logEntryResponse, error) {
	var (
		e            logEntryResponse
		actorID      sql.NullString
		actorLabel   sql.NullString
		resType      sql.NullString
		resID        sql.NullString
		detailsRaw   []byte
		corrID       sql.NullString
		sourceIP     sql.NullString
		sessionID    sql.NullString
		campaignID   sql.NullString
		checksum     sql.NullString
		prevChecksum sql.NullString
		category     string
		severity     string
		actorType    string
	)
	err := rows.Scan(
		&e.ID, &e.Timestamp, &category, &severity, &actorType,
		&actorID, &actorLabel, &e.Action, &resType, &resID,
		&detailsRaw, &corrID, &sourceIP, &sessionID, &campaignID, &checksum,
		&prevChecksum,
	)
	if err != nil {
		return nil, err
	}
	e.Category = category
	e.Severity = severity
	e.ActorType = actorType
	if actorID.Valid {
		s := actorID.String
		e.ActorID = &s
	}
	if actorLabel.Valid {
		e.ActorLabel = actorLabel.String
	}
	if resType.Valid {
		s := resType.String
		e.ResourceType = &s
	}
	if resID.Valid {
		s := resID.String
		e.ResourceID = &s
	}
	if corrID.Valid {
		e.CorrelationID = corrID.String
	}
	if sourceIP.Valid {
		s := sourceIP.String
		e.SourceIP = &s
	}
	if sessionID.Valid {
		s := sessionID.String
		e.SessionID = &s
	}
	if campaignID.Valid {
		s := campaignID.String
		e.CampaignID = &s
	}
	if checksum.Valid {
		e.Checksum = checksum.String
	}
	if prevChecksum.Valid {
		e.PreviousChecksum = prevChecksum.String
	}
	if len(detailsRaw) > 0 {
		if err := json.Unmarshal(detailsRaw, &e.Details); err != nil {
			return nil, err
		}
	}
	return &e, nil
}

// toServiceEntry converts a handler response struct back to a service LogEntry
// (used only for integrity verification).
func toServiceEntry(e *logEntryResponse) *auditsvc.LogEntry {
	entry := &auditsvc.LogEntry{
		ID:               e.ID,
		Timestamp:        e.Timestamp,
		Category:         auditsvc.Category(e.Category),
		Severity:         auditsvc.Severity(e.Severity),
		ActorType:        auditsvc.ActorType(e.ActorType),
		ActorID:          e.ActorID,
		ActorLabel:       e.ActorLabel,
		Action:           e.Action,
		ResourceType:     e.ResourceType,
		ResourceID:       e.ResourceID,
		Details:          e.Details,
		CorrelationID:    e.CorrelationID,
		SourceIP:         e.SourceIP,
		SessionID:        e.SessionID,
		CampaignID:       e.CampaignID,
		Checksum:         e.Checksum,
		PreviousChecksum: e.PreviousChecksum,
	}
	return entry
}
