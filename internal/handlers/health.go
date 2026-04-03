// Package handlers contains HTTP handler functions grouped by domain.
package handlers

import (
	"net/http"

	"tackle/pkg/response"
)

// HealthData is the response body for the health endpoint.
type HealthData struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Health handles GET /api/v1/health.
// It returns a 200 OK with the current status and build version.
func Health(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, HealthData{
			Status:  "ok",
			Version: version,
		})
	}
}
