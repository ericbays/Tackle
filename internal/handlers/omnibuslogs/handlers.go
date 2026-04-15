package omnibuslogs

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type Deps struct {
	DB *sql.DB
}

type OmnibusLogEntry struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Source    string         `json:"source"`
	Severity  string         `json:"severity"`
	Actor     string         `json:"actor"`
	ActionMsg string         `json:"action_msg"`
	Payload   map[string]any `json:"payload"`
}

func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	search := r.URL.Query().Get("search")
	sourceFilter := r.URL.Query().Get("source")
	severityFilter := r.URL.Query().Get("severity")

	// Parse Cursor (timestamp:id) -> Base64
	var cursorTime time.Time
	var cursorID string
	if c := r.URL.Query().Get("cursor"); c != "" {
		decoded, err := base64.StdEncoding.DecodeString(c)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				if t, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
					cursorTime = t
					cursorID = parts[1]
				}
			}
		}
	}

	var args []any
	argIdx := 1

	// Time cursor logic for each sub-query
	timeFilterAudit := ""
	timeFilterApp := ""
	timeFilterEndpoint := ""
	if !cursorTime.IsZero() {
		timeFilterAudit = fmt.Sprintf(" AND (timestamp < $%d OR (timestamp = $%d AND id < $%d)) ", argIdx, argIdx, argIdx+1)
		timeFilterApp = fmt.Sprintf(" AND (timestamp < $%d OR (timestamp = $%d AND id < $%d)) ", argIdx, argIdx, argIdx+1)
		timeFilterEndpoint = fmt.Sprintf(" AND (logged_at < $%d OR (logged_at = $%d AND id < $%d)) ", argIdx, argIdx, argIdx+1)
		args = append(args, cursorTime, cursorID)
		argIdx += 2
	}

	query := fmt.Sprintf(`
		WITH combined_logs AS (
			SELECT 
				id, timestamp, 'audit'::text as source, severity::text, actor_label::text as actor, action::text as action_msg, details as payload 
			FROM audit_logs
			WHERE 1=1 %[1]s
			
			UNION ALL
			
			SELECT 
				id, timestamp, 'application'::text as source, level::text as severity, 'system'::text as actor, message::text as action_msg, attributes as payload 
			FROM app_logs
			WHERE 1=1 %[2]s
			
			UNION ALL
			
			SELECT 
				id, logged_at as timestamp, 'endpoint'::text as source, 'info'::text as severity, source_ip::text as actor, http_method || ' ' || request_path as action_msg, jsonb_build_object('endpoint_id', endpoint_id, 'campaign_id', campaign_id, 'status', response_status) as payload
			FROM endpoint_request_logs
			WHERE 1=1 %[3]s
		)
		SELECT id, timestamp, source, severity, actor, action_msg, payload
		FROM combined_logs
		WHERE 1=1
	`, timeFilterAudit, timeFilterApp, timeFilterEndpoint)

	if sourceFilter != "" {
		query += fmt.Sprintf(" AND source = $%d ", argIdx)
		args = append(args, sourceFilter)
		argIdx++
	}

	if severityFilter != "" {
		query += fmt.Sprintf(" AND severity = $%d ", argIdx)
		args = append(args, severityFilter)
		argIdx++
	}

	if search != "" {
		searchStr := "%" + strings.ToLower(search) + "%"
		query += fmt.Sprintf(" AND (LOWER(actor) LIKE $%d OR LOWER(action_msg) LIKE $%d) ", argIdx, argIdx)
		args = append(args, searchStr)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := d.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query omnibus logs: "+err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var logs []OmnibusLogEntry
	for rows.Next() {
		var l OmnibusLogEntry
		var payloadJSON []byte
		var ts time.Time
		if err := rows.Scan(&l.ID, &ts, &l.Source, &l.Severity, &l.Actor, &l.ActionMsg, &payloadJSON); err != nil {
			continue
		}
		l.Timestamp = ts.Format(time.RFC3339Nano)
		_ = json.Unmarshal(payloadJSON, &l.Payload)
		logs = append(logs, l)
	}

	var nextCursor string
	if len(logs) == limit {
		last := logs[limit-1]
		rawCursor := fmt.Sprintf("%s:%s", last.Timestamp, last.ID)
		nextCursor = base64.StdEncoding.EncodeToString([]byte(rawCursor))
	}

	response.Success(w, map[string]any{
		"data":        logs,
		"next_cursor": nextCursor,
	})
}
