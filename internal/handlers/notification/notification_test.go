package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	authsvc "tackle/internal/services/auth"
	notifsvc "tackle/internal/services/notification"
)

// injectClaims is test middleware that injects JWT claims into the context.
func injectClaims(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := &authsvc.Claims{
				Username: "testuser",
				Role:     "operator",
			}
			claims.Subject = userID
			ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const testUserID = "00000000-0000-0000-0000-000000000001"

func notifColumns() []string {
	return []string{
		"id", "user_id", "category", "severity", "title", "body",
		"resource_type", "resource_id", "action_url", "is_read", "expires_at", "created_at",
	}
}

func newTestDeps(t *testing.T) (*Deps, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	hub := notifsvc.NewHub()
	go hub.Run()

	svc := notifsvc.NewNotificationService(db, hub)

	// JWTService needs a key — use a dummy 32-byte key.
	jwtSvc := authsvc.NewJWTService(make([]byte, 32), 15)

	return &Deps{DB: db, NotifSvc: svc, Hub: hub, JWTSvc: jwtSvc}, mock
}

func TestList_Returns200(t *testing.T) {
	deps, mock := newTestDeps(t)

	ts := time.Now().UTC()
	rows := sqlmock.NewRows(notifColumns()).AddRow(
		"notif-1", testUserID, "system", "info", "Hello", "World",
		nil, nil, nil, false, nil, ts,
	)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, user_id")).WillReturnRows(rows)

	r := chi.NewRouter()
	r.With(injectClaims(testUserID)).Get("/notifications", deps.List)

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []notifsvc.Notification `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Errorf("got %d items, want 1", len(resp.Data))
	}
}

func TestUnreadCount_ReturnsCount(t *testing.T) {
	deps, mock := newTestDeps(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	r := chi.NewRouter()
	r.With(injectClaims(testUserID)).Get("/notifications/unread-count", deps.UnreadCount)

	req := httptest.NewRequest(http.MethodGet, "/notifications/unread-count", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Count != 5 {
		t.Errorf("count = %d, want 5", resp.Data.Count)
	}
}

func TestRead_Returns404ForUnknownID(t *testing.T) {
	deps, mock := newTestDeps(t)

	// UPDATE returns 0 rows affected → 404.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE notifications")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := chi.NewRouter()
	r.With(injectClaims(testUserID)).Put("/notifications/{id}/read", deps.Read)

	req := httptest.NewRequest(http.MethodPut, "/notifications/nonexistent/read", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDelete_Returns204OnSuccess(t *testing.T) {
	deps, mock := newTestDeps(t)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := chi.NewRouter()
	r.With(injectClaims(testUserID)).Delete("/notifications/{id}", deps.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/notifications/notif-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; body: %s", w.Code, w.Body.String())
	}
}

func TestDelete_Returns404WhenNotFound(t *testing.T) {
	deps, mock := newTestDeps(t)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM notifications")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := chi.NewRouter()
	r.With(injectClaims(testUserID)).Delete("/notifications/{id}", deps.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/notifications/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestWS_UpgradeFailsWithoutWebSocketHeaders(t *testing.T) {
	// The new first-message auth pattern upgrades the connection first.
	// Without proper WebSocket headers, the upgrader returns 400.
	deps, _ := newTestDeps(t)

	r := chi.NewRouter()
	r.Get("/ws", deps.WS)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// gorilla/websocket upgrader returns 400 for non-WebSocket requests.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestWS_NoTokenInURL(t *testing.T) {
	// Verify that the handler no longer reads from query parameters.
	// A request with token in the URL should get the same 400 (bad upgrade)
	// as one without — the token is ignored.
	deps, _ := newTestDeps(t)

	r := chi.NewRouter()
	r.Get("/ws", deps.WS)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=some.jwt.value", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (upgrade fails without WS headers)", w.Code)
	}
}
