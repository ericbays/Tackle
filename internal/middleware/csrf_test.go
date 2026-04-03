package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRF_SafeMethodAllowed(t *testing.T) {
	handler := CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Should have set the CSRF cookie and header.
	csrfHeader := rec.Header().Get("X-CSRF-Token")
	if csrfHeader == "" {
		t.Error("expected X-CSRF-Token header on response")
	}
}

func TestCSRF_MutatingWithoutToken_Forbidden(t *testing.T) {
	handler := CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCSRF_MutatingWithValidToken_Allowed(t *testing.T) {
	handler := CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Step 1: GET to obtain the CSRF token.
	getReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	csrfToken := getRec.Header().Get("X-CSRF-Token")
	if csrfToken == "" {
		t.Fatal("no CSRF token returned")
	}

	// Extract the cookie.
	cookies := getRec.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "tackle_csrf" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatal("no CSRF cookie set")
	}

	// Step 2: POST with both the cookie and the header.
	postReq := httptest.NewRequest(http.MethodPost, "/test", nil)
	postReq.AddCookie(csrfCookie)
	postReq.Header.Set("X-CSRF-Token", csrfToken)
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", postRec.Code)
	}
}

func TestCSRF_MismatchedToken_Forbidden(t *testing.T) {
	handler := CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET to get a valid cookie.
	getReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	cookies := getRec.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "tackle_csrf" {
			csrfCookie = c
			break
		}
	}

	// POST with a wrong token.
	postReq := httptest.NewRequest(http.MethodPost, "/test", nil)
	postReq.AddCookie(csrfCookie)
	postReq.Header.Set("X-CSRF-Token", "wrong_token_value")
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", postRec.Code)
	}
}
