package endpointmgmt

import (
	"testing"
	"time"
)

func TestHeartbeatPayload_Validation(t *testing.T) {
	t.Run("empty endpoint_id", func(t *testing.T) {
		payload := HeartbeatPayload{}
		svc := &Service{}
		err := svc.ProcessHeartbeat(nil, payload)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if _, ok := err.(*ValidationError); !ok {
			t.Errorf("expected ValidationError, got %T", err)
		}
	})
}

func TestRequestLogFilter_Defaults(t *testing.T) {
	filter := RequestLogFilter{}
	if filter.Limit != 0 {
		t.Errorf("expected default limit 0 (to be normalized), got %d", filter.Limit)
	}
}

func TestPhishingReportPayload_Validation(t *testing.T) {
	svc := &Service{}

	t.Run("missing reporter_email", func(t *testing.T) {
		_, err := svc.ProcessPhishingReport(nil, PhishingReportPayload{}, "webhook", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if _, ok := err.(*ValidationError); !ok {
			t.Errorf("expected ValidationError, got %T", err)
		}
	})

	t.Run("missing message_id and subject", func(t *testing.T) {
		_, err := svc.ProcessPhishingReport(nil, PhishingReportPayload{
			ReporterEmail: "user@example.com",
		}, "webhook", "")
		if err == nil {
			t.Fatal("expected error")
		}
		if _, ok := err.(*ValidationError); !ok {
			t.Errorf("expected ValidationError, got %T", err)
		}
	})
}

func TestHealthStatus_HeartbeatAge(t *testing.T) {
	now := time.Now()
	past := now.Add(-2 * time.Minute)

	status := HealthStatus{
		LastHeartbeatAt: &past,
		IsHealthy:       true,
	}

	if status.LastHeartbeatAt == nil {
		t.Fatal("LastHeartbeatAt should not be nil")
	}

	age := time.Since(*status.LastHeartbeatAt)
	if age < 2*time.Minute {
		t.Errorf("expected age >= 2m, got %v", age)
	}
}

func TestEndpointDTO_Fields(t *testing.T) {
	dto := EndpointDTO{
		ID:            "test-id",
		CloudProvider: "aws",
		Region:        "us-east-1",
		State:         "active",
	}

	if dto.ID != "test-id" {
		t.Errorf("expected ID test-id, got %s", dto.ID)
	}
	if dto.State != "active" {
		t.Errorf("expected state active, got %s", dto.State)
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ValidationError", &ValidationError{Msg: "bad input"}, "bad input"},
		{"NotFoundError", &NotFoundError{Msg: "not found"}, "not found"},
		{"ConflictError", &ConflictError{Msg: "conflict"}, "conflict"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("expected %q, got %q", tt.msg, tt.err.Error())
			}
		})
	}
}

func TestRequestLogEntry_Struct(t *testing.T) {
	entry := RequestLogEntry{
		SourceIP:       "192.168.1.1",
		HTTPMethod:     "GET",
		RequestPath:    "/login",
		ResponseStatus: 200,
		ResponseSize:   1024,
		ResponseTimeMs: 42,
		TLSVersion:     "TLSv1.3",
	}

	if entry.SourceIP != "192.168.1.1" {
		t.Errorf("unexpected SourceIP: %s", entry.SourceIP)
	}
	if entry.ResponseTimeMs != 42 {
		t.Errorf("unexpected ResponseTimeMs: %d", entry.ResponseTimeMs)
	}
}

func TestTLSCertInfo_Struct(t *testing.T) {
	info := TLSCertInfo{
		ID:          "cert-1",
		EndpointID:  "ep-1",
		Domain:      "phish.example.com",
		Fingerprint: "abc123",
		IsActive:    true,
	}

	if !info.IsActive {
		t.Error("expected IsActive to be true")
	}
	if info.Domain != "phish.example.com" {
		t.Errorf("unexpected domain: %s", info.Domain)
	}
}
