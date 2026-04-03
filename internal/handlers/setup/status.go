package setup

import (
	"log/slog"
	"net/http"

	"tackle/internal/middleware"
	setupsvc "tackle/internal/services/setup"
	"tackle/pkg/response"
)

type setupStatusResponse struct {
	SetupRequired bool `json:"setup_required"`
}

// Status handles GET /api/v1/setup/status.
// Returns whether initial admin setup is still required.
// Public endpoint — no authentication required.
func (d *Deps) Status(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	required, err := setupsvc.IsSetupRequired(r.Context(), d.DB)
	if err != nil {
		slog.Error("setup.status: check failed", "error", err, "correlation_id", correlationID)
		response.Error(w, "INTERNAL_ERROR", "failed to check setup status", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, setupStatusResponse{SetupRequired: required})
}
