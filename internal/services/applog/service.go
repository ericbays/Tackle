package applog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Broadcaster is an interface for broadcasting messages to all WebSocket clients.
type Broadcaster interface {
	BroadcastAll(msg []byte)
}

const (
	defaultBufSize   = 10_000
	defaultBatchSize = 100
	flushInterval    = time.Second
)

type AppLogEntry struct {
	Timestamp  time.Time
	Level      string
	Message    string
	Attributes map[string]any
}

// AppLogService writes application slog entries to PostgreSQL asynchronously.
type AppLogService struct {
	db          *sql.DB
	buf         chan AppLogEntry
	done        chan struct{}
	broadcaster Broadcaster
}

func NewAppLogService(db *sql.DB) *AppLogService {
	s := &AppLogService{
		db:   db,
		buf:  make(chan AppLogEntry, defaultBufSize),
		done: make(chan struct{}),
	}
	go s.worker()
	return s
}

func (s *AppLogService) Log(e AppLogEntry) {
	select {
	case s.buf <- e:
	default:
		// Drop log or sync write if needed? Actually, since this runs in the slog hot path, we do not want to sync-write and lock the entire Go daemon if Postgres is slow. It's safer to drop or log to stderr.
		log.Println("applog: buffer full, dropping log entry")
	}

	s.broadcastEntry(e)
}

func (s *AppLogService) SetBroadcaster(b Broadcaster) {
	s.broadcaster = b
}

type appWSMessage struct {
	Type string         `json:"type"`
	Data appWSPayload   `json:"data"`
}

type appWSPayload struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

func (s *AppLogService) broadcastEntry(e AppLogEntry) {
	if s.broadcaster == nil {
		return
	}
	msg := appWSMessage{
		Type: "app_log_new",
		Data: appWSPayload{
			Timestamp: e.Timestamp.Format(time.RFC3339Nano),
			Level:     e.Level,
			Message:   e.Message,
		},
	}
	payload, err := json.Marshal(msg)
	if err == nil {
		s.broadcaster.BroadcastAll(payload)
	}
}

func (s *AppLogService) worker() {
	defer close(s.done)
	batch := make([]AppLogEntry, 0, defaultBatchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case e, ok := <-s.buf:
			if !ok {
				if len(batch) > 0 {
					s.insertBatch(context.Background(), batch)
				}
				return
			}
			batch = append(batch, e)
			if len(batch) >= defaultBatchSize {
				s.insertBatch(context.Background(), batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.insertBatch(context.Background(), batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *AppLogService) insertBatch(ctx context.Context, batch []AppLogEntry) {
	if len(batch) == 0 {
		return
	}

	valueStrings := make([]string, 0, len(batch))
	valueArgs := make([]any, 0, len(batch)*4)
	i := 1

	for _, e := range batch {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)", i, i+1, i+2, i+3))
		
		attrJSON, _ := json.Marshal(e.Attributes)
		if len(attrJSON) == 0 || string(attrJSON) == "null" {
			attrJSON = []byte("{}")
		}

		valueArgs = append(valueArgs, e.Timestamp, e.Level, e.Message, attrJSON)
		i += 4
	}

	stmt := fmt.Sprintf(`
		INSERT INTO app_logs (timestamp, level, message, attributes) 
		VALUES %s
	`, strings.Join(valueStrings, ","))

	_, err := s.db.ExecContext(ctx, stmt, valueArgs...)
	if err != nil {
		log.Printf("applog: batch insert failed: %v", err)
	}
}

func (s *AppLogService) Drain() {
	close(s.buf)
	<-s.done
}
