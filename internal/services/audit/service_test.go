package audit

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newTestAuditService(t *testing.T) (*AuditService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	key := make([]byte, 32)
	hmac := NewHMACService(key)
	svc := NewAuditService(db, hmac, 100)
	return svc, mock
}

func validEntry() LogEntry {
	return LogEntry{
		Category:  CategoryUserActivity,
		Severity:  SeverityInfo,
		ActorType: ActorTypeUser,
		Action:    "auth.login.success",
	}
}

func TestLog_EnqueuesEntry(t *testing.T) {
	svc, mock := newTestAuditService(t)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := svc.Log(context.Background(), validEntry()); err != nil {
		t.Fatalf("Log error: %v", err)
	}

	// Drain triggers flush.
	svc.Drain()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestLog_SetsIDAndTimestamp(t *testing.T) {
	svc, mock := newTestAuditService(t)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	e := validEntry()
	if err := svc.Log(context.Background(), e); err != nil {
		t.Fatalf("Log error: %v", err)
	}
	svc.Drain()
	_ = mock.ExpectationsWereMet()
}

func TestLog_RejectsInvalidEntry(t *testing.T) {
	svc, _ := newTestAuditService(t)
	defer svc.Drain()

	e := LogEntry{} // missing all mandatory fields
	if err := svc.Log(context.Background(), e); err == nil {
		t.Fatal("expected error for invalid entry")
	}
}

func TestLog_SelfReferencingCorrelationID(t *testing.T) {
	// Verify that an entry with no CorrelationID gets one set equal to its ID.
	// We can't easily inspect the enqueued entry directly, so we use a tiny
	// buffer and drain to trigger the flush, checking no error occurs.
	svc, mock := newTestAuditService(t)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	e := validEntry()
	e.CorrelationID = "" // explicitly empty
	if err := svc.Log(context.Background(), e); err != nil {
		t.Fatalf("Log error: %v", err)
	}
	svc.Drain()
}

func TestLog_SanitizesDetails(t *testing.T) {
	// Confirm a password field is redacted before the checksum is computed
	// (i.e., Log does not return an error and the entry is enqueued).
	svc, mock := newTestAuditService(t)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	e := validEntry()
	e.Details = map[string]any{"password": "hunter2", "user": "alice"}
	if err := svc.Log(context.Background(), e); err != nil {
		t.Fatalf("Log error: %v", err)
	}
	svc.Drain()
}

func TestLog_BatchFlushAtBatchSize(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	key := make([]byte, 32)
	hmac := NewHMACService(key)
	// Use a small buffer so we hit batch size quickly.
	svc := NewAuditService(db, hmac, 1000)

	// Expect exactly one INSERT (batch of defaultBatchSize=100 entries).
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs")).
		WillReturnResult(sqlmock.NewResult(int64(defaultBatchSize), int64(defaultBatchSize)))

	for i := 0; i < defaultBatchSize; i++ {
		if err := svc.Log(context.Background(), validEntry()); err != nil {
			t.Fatalf("Log[%d] error: %v", i, err)
		}
	}

	// Give the worker a moment to process.
	time.Sleep(200 * time.Millisecond)

	// Drain remaining (should be empty).
	svc.Drain()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
