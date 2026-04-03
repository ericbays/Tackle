package endpoints

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/internal/services/endpointmgmt"
)

func TestWriteEndpointError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "validation error",
			err:        &endpointmgmt.ValidationError{Msg: "bad"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "not found error",
			err:        &endpointmgmt.NotFoundError{Msg: "missing"},
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "conflict error",
			err:        &endpointmgmt.ConflictError{Msg: "wrong state"},
			wantStatus: http.StatusConflict,
			wantCode:   "CONFLICT",
		},
		{
			name:       "generic error",
			err:        &errGeneric{},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeEndpointError(w, tt.err, "test-corr")
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			errObj, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatal("missing error object in response")
			}
			if errObj["code"] != tt.wantCode {
				t.Errorf("code = %s, want %s", errObj["code"], tt.wantCode)
			}
		})
	}
}

type errGeneric struct{}

func (e *errGeneric) Error() string { return "generic" }

func TestReceiveHeartbeat_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/endpoint-data/heartbeat", bytes.NewBufferString("invalid"))
	deps.ReceiveHeartbeat(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestReceiveRequestLogs_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/endpoint-data/logs", bytes.NewBufferString("{invalid"))
	deps.ReceiveRequestLogs(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestReceiveRequestLogs_MissingEndpointID(t *testing.T) {
	deps := &Deps{}
	body, _ := json.Marshal(map[string]any{"logs": []any{}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/endpoint-data/logs", bytes.NewBuffer(body))
	deps.ReceiveRequestLogs(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestReceivePhishingReport_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/phishing-reports", bytes.NewBufferString("bad"))
	deps.ReceivePhishingReport(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUploadTLSCertificate_InvalidJSON(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("bad"))
	// No claims context, should fail with 401.
	deps.UploadTLSCertificate(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestStopCampaignEndpoint_Unauthorized(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	deps.StopCampaignEndpoint(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRestartCampaignEndpoint_Unauthorized(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	deps.RestartCampaignEndpoint(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTerminateCampaignEndpoint_Unauthorized(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	deps.TerminateCampaignEndpoint(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRetryCampaignEndpoint_Unauthorized(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	deps.RetryCampaignEndpoint(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestManualPhishingReport_Unauthorized(t *testing.T) {
	deps := &Deps{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	deps.ManualPhishingReport(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
