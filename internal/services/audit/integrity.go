package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"
)

// IntegrityJob runs periodic background verification of audit log checksums.
// It queries audit_logs for entries in the previous interval window, recomputes
// the HMAC for each row, and compares it to the stored checksum.
// Failures are logged at critical severity via slog (not written to audit_logs,
// to avoid a circular dependency).
type IntegrityJob struct {
	db       *sql.DB
	hmac     *HMACService
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewIntegrityJob creates an IntegrityJob. Call Start to begin verification.
func NewIntegrityJob(db *sql.DB, hmac *HMACService, interval time.Duration) *IntegrityJob {
	return &IntegrityJob{
		db:       db,
		hmac:     hmac,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the background verification loop. ctx cancellation also stops the job.
func (j *IntegrityJob) Start(ctx context.Context) {
	go j.run(ctx)
}

// Stop signals the job to stop and waits for the current verification to complete.
func (j *IntegrityJob) Stop() {
	close(j.stop)
	<-j.done
}

func (j *IntegrityJob) run(ctx context.Context) {
	defer close(j.done)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.verify(ctx)
		case <-j.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// verify queries the previous interval window and checks each entry's checksum
// and the hash chain linking consecutive entries.
func (j *IntegrityJob) verify(ctx context.Context) {
	end := time.Now().UTC()
	start := end.Add(-j.interval)

	const q = `
		SELECT id, timestamp, category, severity, actor_type,
		       actor_id, actor_label, action, resource_type, resource_id,
		       details, correlation_id, source_ip, session_id, campaign_id,
		       checksum, previous_checksum
		FROM audit_logs
		WHERE timestamp >= $1 AND timestamp < $2
		ORDER BY timestamp ASC, id ASC`

	rows, err := j.db.QueryContext(ctx, q, start, end)
	if err != nil {
		slog.Error("audit integrity: query failed", "error", err)
		return
	}
	defer rows.Close()

	checked, failed, chainBroken := 0, 0, 0
	var prevChecksum string
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			slog.Error("audit integrity: scan error", "error", err)
			continue
		}
		checked++

		// Verify the entry's own HMAC.
		if !j.hmac.Verify(e) {
			failed++
			slog.Log(ctx, slog.LevelError+4, // critical — above slog.LevelError
				"audit integrity: checksum failure",
				"id", e.ID,
				"action", e.Action,
				"timestamp", e.Timestamp,
			)
		}

		// Verify chain linkage: entry's PreviousChecksum should match the
		// prior entry's Checksum. Skip for the first entry in the window
		// (prevChecksum is empty) or when PreviousChecksum is empty (legacy entry).
		if prevChecksum != "" && e.PreviousChecksum != "" && e.PreviousChecksum != prevChecksum {
			chainBroken++
			slog.Log(ctx, slog.LevelError+4,
				"audit integrity: chain break detected",
				"id", e.ID,
				"expected_previous", prevChecksum,
				"actual_previous", e.PreviousChecksum,
			)
		}
		prevChecksum = e.Checksum
	}
	if err := rows.Err(); err != nil {
		slog.Error("audit integrity: rows error", "error", err)
	}

	slog.Info("audit integrity: verification complete",
		"window_start", start,
		"window_end", end,
		"checked", checked,
		"failed", failed,
		"chain_broken", chainBroken,
	)
}

// scanEntry scans a single audit_logs row into a LogEntry.
func scanEntry(rows *sql.Rows) (*LogEntry, error) {
	var (
		e              LogEntry
		category       string
		severity       string
		actorType      string
		detailsRaw     []byte
		corrID         sql.NullString
		sourceIP       sql.NullString
		sessionID      sql.NullString
		campaignID     sql.NullString
		actorID        sql.NullString
		actorLabel     sql.NullString
		resType        sql.NullString
		resID          sql.NullString
		prevChecksum   sql.NullString
	)
	err := rows.Scan(
		&e.ID, &e.Timestamp, &category, &severity, &actorType,
		&actorID, &actorLabel, &e.Action, &resType, &resID,
		&detailsRaw, &corrID, &sourceIP, &sessionID, &campaignID, &e.Checksum,
		&prevChecksum,
	)
	if err != nil {
		return nil, err
	}
	e.Category = Category(category)
	e.Severity = Severity(severity)
	e.ActorType = ActorType(actorType)
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
