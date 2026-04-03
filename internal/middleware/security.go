// Package middleware provides HTTP middleware for the Tackle server.
package middleware

import "net/http"

// SecurityHeaders returns middleware that adds Content-Security-Policy,
// X-Content-Type-Options, X-Frame-Options, Referrer-Policy, and
// Permissions-Policy headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Content-Security-Policy: restrict script/style/connect/frame sources.
		// 'unsafe-inline' is required for style-src because Tailwind injects
		// inline styles. blob: is needed for landing page builder preview iframes.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"connect-src 'self' wss:; "+
				"frame-src 'self' blob:; "+
				"font-src 'self'; "+
				"object-src 'none'; "+
				"base-uri 'self'")

		// Prevent MIME-type sniffing.
		h.Set("X-Content-Type-Options", "nosniff")

		// Prevent framing by other origins.
		h.Set("X-Frame-Options", "DENY")

		// Control referrer information.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Disable browser features we don't use.
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		next.ServeHTTP(w, r)
	})
}
