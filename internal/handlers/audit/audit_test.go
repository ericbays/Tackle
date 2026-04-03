package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	auditsvc "tackle/internal/services/audit"
)

func newTestDeps(t *testing.T) (*Deps, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	key := make([]byte, 32)
	hmacSvc := auditsvc.NewHMACService(key)
	auditSvc := auditsvc.NewAuditService(db, hmacSvc, 100)
	t.Cleanup(func() { auditSvc.Drain() })
	return &Deps{DB: db, AuditSvc: auditSvc, HMACSvc: hmacSvc}, mock
}

func listColumns() []string {
	return []string{
		"id", "timestamp", "category", "severity", "actor_type",
		"actor_id", "actor_label", "action", "resource_type", "resource_id",
		"details", "correlation_id", "source_ip", "session_id", "campaign_id", "checksum",
		"previous_checksum",
	}
}

func TestList_Returns200(t *testing.T) {
	deps, mock := newTestDeps(t)

	ts := time.Now().UTC()
	rows := sqlmock.NewRows(listColumns()).AddRow(
		"entry-1", ts, "user_activity", "info", "user",
		nil, "alice", "auth.login.success", nil, nil,
		nil, "corr-1", nil, nil, nil, "deadbeef", nil,
	)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, timestamp")).WillReturnRows(rows)

	r := chi.NewRouter()
	r.With(injectAdminClaims).Get("/logs/audit", deps.List)

	req := httptest.NewRequest(http.MethodGet, "/logs/audit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestGet_Returns404ForUnknownID(t *testing.T) {
	deps, mock := newTestDeps(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, timestamp")).
		WillReturnRows(sqlmock.NewRows(listColumns()))

	r := chi.NewRouter()
	r.With(injectAdminClaims).Get("/logs/audit/{id}", deps.Get)

	req := httptest.NewRequest(http.MethodGet, "/logs/audit/nonexistent-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestVerify_Returns200WithValid(t *testing.T) {
	deps, mock := newTestDeps(t)

	// Build an entry with a real checksum.
	key := make([]byte, 32)
	hmacSvc := auditsvc.NewHMACService(key)
	ts := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	e := &auditsvc.LogEntry{
		ID:        "entry-1",
		Timestamp: ts,
		Category:  auditsvc.CategoryUserActivity,
		Severity:  auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser,
		Action:    "auth.login.success",
	}
	checksum, _ := hmacSvc.Compute(e)

	rows := sqlmock.NewRows(listColumns()).AddRow(
		"entry-1", ts, "user_activity", "info", "user",
		nil, "", "auth.login.success", nil, nil,
		nil, "corr-1", nil, nil, nil, checksum, nil,
	)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, timestamp")).WillReturnRows(rows)

	r := chi.NewRouter()
	r.With(injectAdminClaims).Post("/logs/audit/{id}/verify", deps.Verify)

	req := httptest.NewRequest(http.MethodPost, "/logs/audit/entry-1/verify", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Valid bool `json:"valid"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Data.Valid {
		t.Error("expected valid=true")
	}
}
