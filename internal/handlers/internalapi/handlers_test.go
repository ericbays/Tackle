package internalapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/internal/middleware"
)

func TestResolveTrackingToken_NilTokenSvc(t *testing.T) {
	deps := &Deps{}
	r := httptest.NewRequest(http.MethodPost, "/internal/captures", nil)

	campaignID, targetID, _, isUnattributed := deps.resolveTrackingToken(r, "some-token", "build-campaign-1")
	if !isUnattributed {
		t.Error("expected unattributed when TokenSvc is nil")
	}
	if campaignID != "build-campaign-1" {
		t.Errorf("expected campaign from buildInfo, got %q", campaignID)
	}
	if targetID != "" {
		t.Errorf("expected empty targetID, got %q", targetID)
	}
}

func TestResolveTrackingToken_EmptyToken(t *testing.T) {
	deps := &Deps{}
	r := httptest.NewRequest(http.MethodPost, "/internal/captures", nil)

	campaignID, targetID, _, isUnattributed := deps.resolveTrackingToken(r, "", "build-campaign-1")
	if !isUnattributed {
		t.Error("expected unattributed when token is empty")
	}
	if campaignID != "build-campaign-1" {
		t.Errorf("expected campaign from buildInfo, got %q", campaignID)
	}
	if targetID != "" {
		t.Errorf("expected empty targetID, got %q", targetID)
	}
}

func TestEndpointIDFromContext(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/internal/captures", nil)
	ctx := middleware.WithEndpointAuth(r.Context(), "ep-123")
	r = r.WithContext(ctx)

	endpointID := middleware.EndpointAuthFromContext(r.Context())
	if endpointID != "ep-123" {
		t.Errorf("expected endpoint ID 'ep-123', got %q", endpointID)
	}
}

func TestEndpointIDFromContext_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/internal/captures", nil)

	endpointID := middleware.EndpointAuthFromContext(r.Context())
	if endpointID != "" {
		t.Errorf("expected empty endpoint ID without context, got %q", endpointID)
	}
}

func TestHandleCapture_MethodNotAllowed(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/captures", nil)

	deps.HandleCapture(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleCapture_MissingBuildToken(t *testing.T) {
	deps := &Deps{}
	body, _ := json.Marshal(CapturePayload{
		Fields:        map[string]any{"username": "test"},
		TrackingToken: "tok",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/internal/captures", bytes.NewReader(body))

	deps.HandleCapture(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleTracking_MethodNotAllowed(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/tracking", nil)

	deps.HandleTracking(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleTracking_MissingBuildToken(t *testing.T) {
	deps := &Deps{}
	body, _ := json.Marshal(TrackingPayload{
		TrackingToken: "tok",
		EventType:     "page_view",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/internal/tracking", bytes.NewReader(body))

	deps.HandleTracking(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleTelemetry_MethodNotAllowed(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/telemetry", nil)

	deps.HandleTelemetry(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleTelemetry_MissingBuildToken(t *testing.T) {
	deps := &Deps{}
	body, _ := json.Marshal(TelemetryPayload{
		TrackingToken: "tok",
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/internal/telemetry", bytes.NewReader(body))

	deps.HandleTelemetry(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleDeliveryResult_MethodNotAllowed(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/delivery-result", nil)

	deps.HandleDeliveryResult(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
