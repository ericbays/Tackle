package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Broadcaster is an interface for broadcasting messages to all WebSocket clients.
type Broadcaster interface {
	BroadcastAll(msg []byte)
}

const (
	defaultBufSize   = 10_000
	defaultBatchSize = 100
	flushInterval    = time.Second
	flushThreshold   = 0.8
)

// AuditService writes audit log entries to PostgreSQL asynchronously.
// Entries are buffered in memory and flushed in batches.
// Never fails a caller's request — errors are logged to stdout only.
type AuditService struct {
	db      *sql.DB
	hmac    *HMACService
	buf     chan LogEntry
	done    chan struct{}
	bufSize int

	// chainMu protects lastChecksum for HMAC chain continuity.
	chainMu      sync.Mutex
	lastChecksum string

	// broadcaster sends new audit entries to all WebSocket clients (optional).
	broadcaster Broadcaster

	// alertEvaluator checks entries against alert rules (optional).
	alertEvaluator *AlertEvaluator
}

// NewAuditService creates an AuditService and starts the background writer goroutine.
// bufSize is the capacity of the in-memory channel buffer (0 → default 10,000).
func NewAuditService(db *sql.DB, hmac *HMACService, bufSize int) *AuditService {
	if bufSize <= 0 {
		bufSize = defaultBufSize
	}
	s := &AuditService{
		db:      db,
		hmac:    hmac,
		buf:     make(chan LogEntry, bufSize),
		done:    make(chan struct{}),
		bufSize: bufSize,
	}
	go s.worker()
	return s
}

// Log validates, sanitizes, checksums, and enqueues a log entry.
// The entry's HMAC includes the previous entry's checksum to form a hash chain.
// If the buffer is full, it writes synchronously to avoid dropping entries.
// Returns immediately in the common case (buffer has space).
func (s *AuditService) Log(ctx context.Context, e LogEntry) error {
	if err := e.Validate(); err != nil {
		return err
	}

	// Set ID and Timestamp.
	e.ID = uuid.New().String()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	// Self-referencing correlation ID for standalone entries.
	if e.CorrelationID == "" {
		e.CorrelationID = e.ID
	}

	// Sanitize details before checksum (checksum covers sanitized data).
	e.Details = SanitizeDetails(e.Details)

	// Link to previous entry's checksum for the hash chain.
	s.chainMu.Lock()
	e.PreviousChecksum = s.lastChecksum

	// Compute HMAC checksum (includes previous_checksum in the hash input).
	checksum, err := s.hmac.Compute(&e)
	if err != nil {
		s.chainMu.Unlock()
		return fmt.Errorf("audit: compute checksum: %w", err)
	}
	e.Checksum = checksum
	s.lastChecksum = checksum
	s.chainMu.Unlock()

	// Enqueue or write synchronously if buffer is at/near capacity.
	select {
	case s.buf <- e:
		// Fast path: buffer has space.
	default:
		// Back-pressure: buffer full — write synchronously.
		if err := s.insertBatch(ctx, []LogEntry{e}); err != nil {
			slog.Error("audit: synchronous write failed", "error", err)
		}
	}

	// Broadcast to WebSocket clients for live tail.
	s.broadcastEntry(e)

	// Evaluate alert rules asynchronously.
	if s.alertEvaluator != nil {
		go s.alertEvaluator.Evaluate(e)
	}

	return nil
}

// SetBroadcaster configures the WebSocket broadcaster for live log streaming.
func (s *AuditService) SetBroadcaster(b Broadcaster) {
	s.broadcaster = b
}

// SetAlertEvaluator configures the alert evaluator for rule-based alerting.
func (s *AuditService) SetAlertEvaluator(ae *AlertEvaluator) {
	s.alertEvaluator = ae
}

// auditWSMessage is the WebSocket envelope for a new audit log entry.
type auditWSMessage struct {
	Type string         `json:"type"`
	Data auditWSPayload `json:"data"`
}

// auditWSPayload is the minimal payload broadcast for new audit entries.
type auditWSPayload struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Category   string `json:"category"`
	Severity   string `json:"severity"`
	Action     string `json:"action"`
	ActorLabel string `json:"actor_label"`
}

// broadcastEntry sends a minimal audit entry summary to all WebSocket clients.
func (s *AuditService) broadcastEntry(e LogEntry) {
	if s.broadcaster == nil {
		return
	}
	msg := auditWSMessage{
		Type: "audit_log_new",
		Data: auditWSPayload{
			ID:         e.ID,
			Timestamp:  e.Timestamp.Format(time.RFC3339Nano),
			Category:   string(e.Category),
			Severity:   string(e.Severity),
			Action:     e.Action,
			ActorLabel: e.ActorLabel,
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("audit: marshal ws broadcast", "error", err)
		return
	}
	s.broadcaster.BroadcastAll(payload)
}

// Drain flushes all buffered entries to the database and stops the background worker.
// Call this during application shutdown and wait for it to return before exiting.
func (s *AuditService) Drain() {
	close(s.buf)
	<-s.done
}

// worker is the background goroutine that reads from the buffer and batch-inserts.
func (s *AuditService) worker() {
	defer close(s.done)

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]LogEntry, 0, defaultBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.insertBatch(ctx, batch); err != nil {
			slog.Error("audit: batch insert failed", "count", len(batch), "error", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case e, ok := <-s.buf:
			if !ok {
				// Channel closed — drain remaining entries.
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= defaultBatchSize {
				flush()
				continue
			}
			// Flush at 80% buffer capacity.
			if cap(s.buf) > 0 && float64(len(s.buf))/float64(cap(s.buf)) >= flushThreshold {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// insertBatch bulk-inserts a slice of log entries in a single parameterised statement.
func (s *AuditService) insertBatch(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	const cols = 17
	placeholders := make([]string, 0, len(entries))
	args := make([]any, 0, len(entries)*cols)

	for i, e := range entries {
		base := i * cols
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d::jsonb,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
			base+8, base+9, base+10, base+11, base+12, base+13, base+14, base+15, base+16, base+17,
		))

		detailsJSON, err := marshalDetails(e.Details)
		if err != nil {
			return fmt.Errorf("audit: marshal details for insert: %w", err)
		}
		var detailsArg any
		if detailsJSON == "null" {
			detailsArg = nil
		} else {
			detailsArg = detailsJSON
		}

		var prevChecksum any
		if e.PreviousChecksum != "" {
			prevChecksum = e.PreviousChecksum
		}

		args = append(args,
			e.ID,
			e.Timestamp.UTC(),
			string(e.Category),
			string(e.Severity),
			string(e.ActorType),
			nullableUUIDPtr(e.ActorID),
			e.ActorLabel,
			e.Action,
			nullableStrPtr(e.ResourceType),
			nullableUUIDPtr(e.ResourceID),
			detailsArg,
			nullableUUID(e.CorrelationID),
			nullableStrPtr(e.SourceIP),
			nullableUUIDPtr(e.SessionID),
			nullableUUIDPtr(e.CampaignID),
			e.Checksum,
			prevChecksum,
		)
	}

	query := `INSERT INTO audit_logs
		(id, timestamp, category, severity, actor_type, actor_id, actor_label, action,
		 resource_type, resource_id, details, correlation_id, source_ip, session_id, campaign_id, checksum, previous_checksum)
		VALUES ` + strings.Join(placeholders, ",")

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("audit: insert batch: %w", err)
	}
	return nil
}

// nullableUUID returns nil for an empty string (so the DB gets NULL), otherwise the string.
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableUUIDPtr returns nil if the pointer is nil, empty, or an invalid format like "system".
func nullableUUIDPtr(s *string) any {
	if s == nil || *s == "" || *s == "system" {
		return nil
	}
	return *s
}

// nullableStrPtr returns nil for nil or empty string pointers.
func nullableStrPtr(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}
