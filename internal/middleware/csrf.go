package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"tackle/pkg/response"
)

const (
	csrfTokenHeader = "X-CSRF-Token"
	csrfCookieName  = "tackle_csrf"
	csrfTokenBytes  = 32 // 256-bit token
)

// CSRFProtection returns middleware that enforces CSRF double-submit cookie
// validation on state-changing requests (POST, PUT, DELETE, PATCH).
//
// On every request, if no CSRF cookie exists, one is generated and set.
// The token value is also returned in the X-CSRF-Token response header so the
// frontend can read it and include it in subsequent mutating requests.
//
// API-key-authenticated requests are exempt (no cookies = no CSRF risk).
// GET, HEAD, and OPTIONS requests are exempt (safe methods).
func CSRFProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API key auth is immune to CSRF — skip validation entirely.
		if IsAPIKeyAuth(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}

		// Read or generate the CSRF cookie token.
		cookieToken := ""
		if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
			cookieToken = c.Value
		}
		if cookieToken == "" {
			cookieToken = generateCSRFToken()
		}

		// Always set/refresh the cookie and expose the token in a response header.
		http.SetCookie(w, &http.Cookie{
			Name:     csrfCookieName,
			Value:    cookieToken,
			Path:     "/",
			HttpOnly: false, // must be readable by JS
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
		w.Header().Set(csrfTokenHeader, cookieToken)

		// Safe methods don't require CSRF validation.
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Mutating request — require the header to match the cookie.
		headerToken := r.Header.Get(csrfTokenHeader)
		if headerToken == "" || headerToken != cookieToken {
			correlationID := GetCorrelationID(r.Context())
			response.Error(w, "CSRF_VALIDATION_FAILED", "missing or invalid CSRF token", http.StatusForbidden, correlationID)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateCSRFToken returns a cryptographically random hex-encoded token.
func generateCSRFToken() string {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; rand.Read uses /dev/urandom.
		panic("csrf: failed to generate random token")
	}
	return hex.EncodeToString(b)
}
