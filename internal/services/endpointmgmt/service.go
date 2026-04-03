// Package endpointmgmt implements the endpoint management service layer:
// heartbeat processing, health monitoring, lifecycle operations, request log ingestion,
// TLS certificate management, phishing report tracking, and error recovery.
package endpointmgmt

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"tackle/internal/crypto"
	"tackle/internal/endpoint"
	"tackle/internal/endpoint/cloud"
	"tackle/internal/repositories"
	"tackle/internal/services/audit"
	"tackle/internal/services/notification"
)

// Service provides endpoint management operations.
type Service struct {
	db       *sql.DB
	repo     *repositories.PhishingEndpointRepository
	sm       *endpoint.StateMachine
	prov     *endpoint.Provisioner
	commAuth *endpoint.CommAuthService
	encSvc   *crypto.EncryptionService
	auditSvc *audit.AuditService
	notifSvc *notification.NotificationService
	hub      *notification.Hub

	// Health monitoring state.
	mu                    sync.RWMutex
	heartbeatInterval     time.Duration
	heartbeatMissThreshold int
}

// NewService creates a new endpoint management Service.
func NewService(
	db *sql.DB,
	repo *repositories.PhishingEndpointRepository,
	sm *endpoint.StateMachine,
	prov *endpoint.Provisioner,
	commAuth *endpoint.CommAuthService,
	encSvc *crypto.EncryptionService,
	auditSvc *audit.AuditService,
	notifSvc *notification.NotificationService,
) *Service {
	return &Service{
		db:                     db,
		repo:                   repo,
		sm:                     sm,
		prov:                   prov,
		commAuth:               commAuth,
		encSvc:                 encSvc,
		auditSvc:               auditSvc,
		notifSvc:               notifSvc,
		heartbeatInterval:      30 * time.Second,
		heartbeatMissThreshold: 3,
	}
}

// SetHub sets the WebSocket hub for broadcasting endpoint status updates.
func (s *Service) SetHub(hub *notification.Hub) {
	s.hub = hub
}

// broadcastEndpointEvent sends an endpoint event to all connected WebSocket clients.
func (s *Service) broadcastEndpointEvent(eventType string, data any) {
	if s.hub == nil {
		return
	}
	msg := map[string]any{
		"type": eventType,
		"data": data,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("broadcast endpoint event: marshal", "error", err)
		return
	}
	s.hub.BroadcastAll(payload)
}

// ValidationError indicates invalid input.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError indicates a resource was not found.
type NotFoundError struct{ Msg string }

func (e *NotFoundError) Error() string { return e.Msg }

// ConflictError indicates a state conflict.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// --- MGMT-06: Heartbeat Processing ---

// HeartbeatPayload represents the data sent by the endpoint binary in each heartbeat.
type HeartbeatPayload struct {
	EndpointID       string    `json:"endpoint_id"`
	CampaignID       string    `json:"campaign_id"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	CPUUsagePct      float64   `json:"cpu_usage_pct"`
	MemoryUsedBytes  int64     `json:"memory_used_bytes"`
	MemoryTotalBytes int64     `json:"memory_total_bytes"`
	DiskUsedBytes    int64     `json:"disk_used_bytes"`
	DiskTotalBytes   int64     `json:"disk_total_bytes"`
	ActiveConnections int      `json:"active_connections"`
	TotalRequests    int64     `json:"total_requests"`
	TotalEmails      int64     `json:"total_emails"`
	LogBufferDepth   int       `json:"log_buffer_depth"`
	Timestamp        time.Time `json:"timestamp"`
}

// ProcessHeartbeat validates and stores a heartbeat from an endpoint.
func (s *Service) ProcessHeartbeat(ctx context.Context, payload HeartbeatPayload) error {
	if payload.EndpointID == "" {
		return &ValidationError{Msg: "endpoint_id is required"}
	}

	// Verify endpoint exists and is active.
	ep, err := s.repo.GetByID(ctx, payload.EndpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NotFoundError{Msg: "endpoint not found"}
		}
		return fmt.Errorf("process heartbeat: get endpoint: %w", err)
	}

	if ep.State != repositories.EndpointStateActive {
		return &ConflictError{Msg: fmt.Sprintf("endpoint is in %s state, expected active", ep.State)}
	}

	// Update last_heartbeat_at timestamp on the endpoint record.
	if err := s.repo.UpdateHeartbeat(ctx, payload.EndpointID); err != nil {
		return fmt.Errorf("process heartbeat: update timestamp: %w", err)
	}

	// Store the heartbeat data for time-series queries.
	reportedAt := payload.Timestamp
	if reportedAt.IsZero() {
		reportedAt = time.Now().UTC()
	}

	var campaignID *string
	if payload.CampaignID != "" {
		campaignID = &payload.CampaignID
	} else {
		campaignID = ep.CampaignID
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO endpoint_heartbeats
			(endpoint_id, campaign_id, uptime_seconds, cpu_usage_pct,
			 memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes,
			 active_connections, total_requests, total_emails, log_buffer_depth, reported_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		payload.EndpointID, campaignID, payload.UptimeSeconds, payload.CPUUsagePct,
		payload.MemoryUsedBytes, payload.MemoryTotalBytes,
		payload.DiskUsedBytes, payload.DiskTotalBytes,
		payload.ActiveConnections, payload.TotalRequests, payload.TotalEmails,
		payload.LogBufferDepth, reportedAt,
	)
	if err != nil {
		return fmt.Errorf("process heartbeat: insert: %w", err)
	}

	// Broadcast health update to WebSocket clients.
	s.broadcastEndpointEvent("endpoint_health_update", map[string]any{
		"endpoint_id":      payload.EndpointID,
		"cpu_usage_pct":    payload.CPUUsagePct,
		"memory_used_bytes": payload.MemoryUsedBytes,
		"memory_total_bytes": payload.MemoryTotalBytes,
		"disk_used_bytes":  payload.DiskUsedBytes,
		"disk_total_bytes": payload.DiskTotalBytes,
		"active_connections": payload.ActiveConnections,
		"uptime_seconds":   payload.UptimeSeconds,
		"timestamp":        reportedAt,
	})

	return nil
}

// --- MGMT-07: Health Monitoring ---

// HealthStatus represents the health status of an endpoint.
type HealthStatus struct {
	EndpointID      string     `json:"endpoint_id"`
	State           string     `json:"state"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`
	HeartbeatAge    string     `json:"heartbeat_age,omitempty"`
	IsHealthy       bool       `json:"is_healthy"`
	CPUUsagePct     float64    `json:"cpu_usage_pct"`
	MemoryUsedPct   float64    `json:"memory_used_pct"`
	DiskUsedPct     float64    `json:"disk_used_pct"`
	ActiveConns     int        `json:"active_connections"`
	TotalRequests   int64      `json:"total_requests"`
	Uptime          int64      `json:"uptime_seconds"`
}

// HeartbeatRecord is a single heartbeat entry for time-series queries.
type HeartbeatRecord struct {
	ID                string    `json:"id"`
	EndpointID        string    `json:"endpoint_id"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	CPUUsagePct       float64   `json:"cpu_usage_pct"`
	MemoryUsedBytes   int64     `json:"memory_used_bytes"`
	MemoryTotalBytes  int64     `json:"memory_total_bytes"`
	DiskUsedBytes     int64     `json:"disk_used_bytes"`
	DiskTotalBytes    int64     `json:"disk_total_bytes"`
	ActiveConnections int       `json:"active_connections"`
	TotalRequests     int64     `json:"total_requests"`
	TotalEmails       int64     `json:"total_emails"`
	LogBufferDepth    int       `json:"log_buffer_depth"`
	ReportedAt        time.Time `json:"reported_at"`
}

// GetHealthStatus returns the current health status for an endpoint.
func (s *Service) GetHealthStatus(ctx context.Context, endpointID string) (HealthStatus, error) {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return HealthStatus{}, &NotFoundError{Msg: "endpoint not found"}
		}
		return HealthStatus{}, fmt.Errorf("get health status: %w", err)
	}

	status := HealthStatus{
		EndpointID:      ep.ID,
		State:           string(ep.State),
		LastHeartbeatAt: ep.LastHeartbeatAt,
		IsHealthy:       ep.State == repositories.EndpointStateActive,
	}

	// Calculate heartbeat age.
	if ep.LastHeartbeatAt != nil {
		age := time.Since(*ep.LastHeartbeatAt)
		status.HeartbeatAge = age.Round(time.Second).String()
		threshold := s.heartbeatInterval * time.Duration(s.heartbeatMissThreshold)
		if age > threshold {
			status.IsHealthy = false
		}
	} else if ep.State == repositories.EndpointStateActive {
		status.IsHealthy = false // Active but never sent a heartbeat.
	}

	// Get latest heartbeat data for resource usage.
	var hb HeartbeatRecord
	err = s.db.QueryRowContext(ctx, `
		SELECT uptime_seconds, cpu_usage_pct, memory_used_bytes, memory_total_bytes,
		       disk_used_bytes, disk_total_bytes, active_connections, total_requests
		FROM endpoint_heartbeats
		WHERE endpoint_id = $1
		ORDER BY reported_at DESC LIMIT 1`, endpointID).Scan(
		&hb.UptimeSeconds, &hb.CPUUsagePct, &hb.MemoryUsedBytes, &hb.MemoryTotalBytes,
		&hb.DiskUsedBytes, &hb.DiskTotalBytes, &hb.ActiveConnections, &hb.TotalRequests,
	)
	if err == nil {
		status.CPUUsagePct = hb.CPUUsagePct
		status.ActiveConns = hb.ActiveConnections
		status.TotalRequests = hb.TotalRequests
		status.Uptime = hb.UptimeSeconds
		if hb.MemoryTotalBytes > 0 {
			status.MemoryUsedPct = float64(hb.MemoryUsedBytes) / float64(hb.MemoryTotalBytes) * 100
		}
		if hb.DiskTotalBytes > 0 {
			status.DiskUsedPct = float64(hb.DiskUsedBytes) / float64(hb.DiskTotalBytes) * 100
		}
	}

	return status, nil
}

// GetHealthHistory returns heartbeat time-series data for an endpoint.
func (s *Service) GetHealthHistory(ctx context.Context, endpointID string, since time.Time, limit int) ([]HeartbeatRecord, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, endpoint_id, uptime_seconds, cpu_usage_pct,
		       memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes,
		       active_connections, total_requests, total_emails, log_buffer_depth, reported_at
		FROM endpoint_heartbeats
		WHERE endpoint_id = $1 AND reported_at >= $2
		ORDER BY reported_at DESC
		LIMIT $3`, endpointID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get health history: %w", err)
	}
	defer rows.Close()

	var records []HeartbeatRecord
	for rows.Next() {
		var r HeartbeatRecord
		if err := rows.Scan(
			&r.ID, &r.EndpointID, &r.UptimeSeconds, &r.CPUUsagePct,
			&r.MemoryUsedBytes, &r.MemoryTotalBytes, &r.DiskUsedBytes, &r.DiskTotalBytes,
			&r.ActiveConnections, &r.TotalRequests, &r.TotalEmails, &r.LogBufferDepth,
			&r.ReportedAt,
		); err != nil {
			return nil, fmt.Errorf("get health history: scan: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// CheckHeartbeatTimeouts scans active endpoints for heartbeat timeout and transitions them to Error.
// Called periodically by a background worker.
func (s *Service) CheckHeartbeatTimeouts(ctx context.Context) error {
	endpoints, err := s.repo.ListByState(ctx, repositories.EndpointStateActive)
	if err != nil {
		return fmt.Errorf("check heartbeat timeouts: %w", err)
	}

	threshold := s.heartbeatInterval * time.Duration(s.heartbeatMissThreshold)

	for _, ep := range endpoints {
		if ep.LastHeartbeatAt == nil {
			// Never received a heartbeat; check how long since activation.
			if ep.ActivatedAt != nil && time.Since(*ep.ActivatedAt) > threshold {
				s.handleHeartbeatTimeout(ctx, ep)
			}
			continue
		}

		if time.Since(*ep.LastHeartbeatAt) > threshold {
			s.handleHeartbeatTimeout(ctx, ep)
		}
	}

	return nil
}

func (s *Service) handleHeartbeatTimeout(ctx context.Context, ep repositories.PhishingEndpoint) {
	reason := fmt.Sprintf("heartbeat timeout: no heartbeat received in %v", s.heartbeatInterval*time.Duration(s.heartbeatMissThreshold))
	_, err := s.sm.TransitionSystem(ctx, ep.ID, repositories.EndpointStateError, reason)
	if err != nil {
		slog.Error("heartbeat timeout: transition to error failed", "endpoint_id", ep.ID, "error", err)
		return
	}

	// Send notification (MGMT-11).
	s.notifyEndpointError(ctx, ep, "Heartbeat Timeout", reason)

	slog.Warn("heartbeat timeout: endpoint transitioned to error", "endpoint_id", ep.ID)
}

// CheckHeartbeatTimeoutsDetailed returns lists of timed-out and healthy endpoint IDs
// without transitioning state — the caller (health worker) manages consecutive failure tracking.
func (s *Service) CheckHeartbeatTimeoutsDetailed(ctx context.Context) (timedOut []string, healthy []string, err error) {
	endpoints, err := s.repo.ListByState(ctx, repositories.EndpointStateActive)
	if err != nil {
		return nil, nil, fmt.Errorf("check heartbeat timeouts: %w", err)
	}

	threshold := s.heartbeatInterval * time.Duration(s.heartbeatMissThreshold)

	for _, ep := range endpoints {
		isTimedOut := false

		if ep.LastHeartbeatAt == nil {
			if ep.ActivatedAt != nil && time.Since(*ep.ActivatedAt) > threshold {
				isTimedOut = true
			}
		} else if time.Since(*ep.LastHeartbeatAt) > threshold {
			isTimedOut = true
		}

		if isTimedOut {
			timedOut = append(timedOut, ep.ID)
		} else {
			healthy = append(healthy, ep.ID)
		}
	}

	return timedOut, healthy, nil
}

// TransitionToError transitions an endpoint to the error state and sends a notification.
func (s *Service) TransitionToError(ctx context.Context, endpointID, reason string) error {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("transition to error: get endpoint: %w", err)
	}

	_, err = s.sm.TransitionSystem(ctx, endpointID, repositories.EndpointStateError, reason)
	if err != nil {
		return fmt.Errorf("transition to error: %w", err)
	}

	s.notifyEndpointError(ctx, ep, "Health Check Failure", reason)

	// Broadcast state change to WebSocket clients.
	s.broadcastEndpointEvent("endpoint_state_change", map[string]any{
		"endpoint_id": endpointID,
		"old_state":   ep.State,
		"new_state":   string(repositories.EndpointStateError),
		"reason":      reason,
	})

	return nil
}

// --- MGMT-08: Stop and Restart ---

// StopEndpoint gracefully stops an active endpoint: halts proxy binary and stops cloud instance.
func (s *Service) StopEndpoint(ctx context.Context, endpointID, actor string, provider cloud.Provider) error {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NotFoundError{Msg: "endpoint not found"}
		}
		return fmt.Errorf("stop endpoint: %w", err)
	}

	if ep.State != repositories.EndpointStateActive {
		return &ConflictError{Msg: fmt.Sprintf("cannot stop endpoint in %s state", ep.State)}
	}

	// Stop cloud instance.
	if ep.InstanceID != nil && *ep.InstanceID != "" {
		if err := provider.StopInstance(ctx, *ep.InstanceID); err != nil {
			slog.Error("stop endpoint: cloud stop failed", "error", err, "endpoint_id", endpointID)
			return fmt.Errorf("stop endpoint: cloud stop: %w", err)
		}
	}

	// Transition to Stopped.
	_, err = s.sm.Transition(ctx, endpointID, repositories.EndpointStateStopped, actor, "operator stopped endpoint")
	if err != nil {
		return fmt.Errorf("stop endpoint: transition: %w", err)
	}

	resourceType := "phishing_endpoint"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.stopped",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
	})

	s.broadcastEndpointEvent("endpoint_state_change", map[string]any{
		"endpoint_id": endpointID,
		"old_state":   string(ep.State),
		"new_state":   string(repositories.EndpointStateStopped),
	})

	return nil
}

// RestartEndpoint restarts a stopped endpoint: starts cloud instance and proxy binary.
func (s *Service) RestartEndpoint(ctx context.Context, endpointID, actor string, provider cloud.Provider) error {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NotFoundError{Msg: "endpoint not found"}
		}
		return fmt.Errorf("restart endpoint: %w", err)
	}

	if ep.State != repositories.EndpointStateStopped {
		return &ConflictError{Msg: fmt.Sprintf("cannot restart endpoint in %s state", ep.State)}
	}

	// Start cloud instance.
	if ep.InstanceID != nil && *ep.InstanceID != "" {
		if err := provider.StartInstance(ctx, *ep.InstanceID); err != nil {
			slog.Error("restart endpoint: cloud start failed", "error", err, "endpoint_id", endpointID)
			return fmt.Errorf("restart endpoint: cloud start: %w", err)
		}
	}

	// Transition to Active.
	_, err = s.sm.Transition(ctx, endpointID, repositories.EndpointStateActive, actor, "operator restarted endpoint")
	if err != nil {
		return fmt.Errorf("restart endpoint: transition: %w", err)
	}

	resourceType := "phishing_endpoint"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.restarted",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
	})

	s.broadcastEndpointEvent("endpoint_state_change", map[string]any{
		"endpoint_id": endpointID,
		"old_state":   string(ep.State),
		"new_state":   string(repositories.EndpointStateActive),
	})

	return nil
}

// --- MGMT-09: Endpoint Termination ---

// TerminateEndpoint runs the full termination workflow: graceful shutdown, VM termination,
// IP release, DNS cleanup, SSH key destruction, credential invalidation.
func (s *Service) TerminateEndpoint(ctx context.Context, endpointID, actor string, provider cloud.Provider, dnsUpdater endpoint.DNSUpdater, zone, subdomain string) error {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NotFoundError{Msg: "endpoint not found"}
		}
		return fmt.Errorf("terminate endpoint: %w", err)
	}

	// Only Active, Stopped, and Error states can be terminated.
	if ep.State != repositories.EndpointStateActive &&
		ep.State != repositories.EndpointStateStopped &&
		ep.State != repositories.EndpointStateError {
		return &ConflictError{Msg: fmt.Sprintf("cannot terminate endpoint in %s state", ep.State)}
	}

	// Invalidate communication credentials.
	if err := s.commAuth.InvalidateCredentials(ctx, endpointID); err != nil {
		slog.Warn("terminate endpoint: credential invalidation failed", "error", err, "endpoint_id", endpointID)
	}

	// Destroy SSH key if present.
	if ep.SSHKeyID != nil && *ep.SSHKeyID != "" {
		if err := s.repo.DestroySSHKey(ctx, *ep.SSHKeyID); err != nil {
			slog.Warn("terminate endpoint: ssh key destruction failed", "error", err, "endpoint_id", endpointID)
		}
	}

	// Use provisioner teardown for cloud resources + DNS + state transition.
	if err := s.prov.TeardownEndpoint(ctx, provider, dnsUpdater, endpointID, zone, subdomain, actor); err != nil {
		return fmt.Errorf("terminate endpoint: teardown: %w", err)
	}

	return nil
}

// --- MGMT-10: Automatic Cleanup on Campaign Completion ---

// HandleCampaignCompletion processes endpoint cleanup when a campaign completes or is cancelled.
// If autoTerminate is true, the endpoint is terminated. Otherwise it is stopped.
func (s *Service) HandleCampaignCompletion(ctx context.Context, campaignID, actor string, autoTerminate bool, provider cloud.Provider, dnsUpdater endpoint.DNSUpdater) error {
	endpoints, err := s.repo.ListByCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("campaign completion: list endpoints: %w", err)
	}

	for _, ep := range endpoints {
		if ep.State == repositories.EndpointStateTerminated {
			continue
		}

		if autoTerminate {
			if err := s.TerminateEndpoint(ctx, ep.ID, actor, provider, dnsUpdater, "", ""); err != nil {
				slog.Error("campaign completion: terminate endpoint failed", "error", err, "endpoint_id", ep.ID)
			}
		} else {
			// Stop the endpoint but keep it for later manual cleanup.
			if ep.State == repositories.EndpointStateActive {
				if err := s.StopEndpoint(ctx, ep.ID, actor, provider); err != nil {
					slog.Error("campaign completion: stop endpoint failed", "error", err, "endpoint_id", ep.ID)
				}
			}
		}
	}

	return nil
}

// ListAllEndpoints returns all endpoints across campaigns with optional state filter.
func (s *Service) ListAllEndpoints(ctx context.Context, stateFilter string) ([]repositories.PhishingEndpoint, error) {
	if stateFilter != "" {
		return s.repo.ListByState(ctx, repositories.EndpointState(stateFilter))
	}

	// List all non-terminated endpoints.
	var results []repositories.PhishingEndpoint
	for _, state := range []repositories.EndpointState{
		repositories.EndpointStateRequested,
		repositories.EndpointStateProvisioning,
		repositories.EndpointStateConfiguring,
		repositories.EndpointStateActive,
		repositories.EndpointStateStopped,
		repositories.EndpointStateError,
	} {
		eps, err := s.repo.ListByState(ctx, state)
		if err != nil {
			return nil, fmt.Errorf("list all endpoints: %w", err)
		}
		results = append(results, eps...)
	}
	return results, nil
}

// --- MGMT-04: Request Logging ---

// RequestLogEntry represents a single proxied request log from the endpoint.
type RequestLogEntry struct {
	EndpointID      string            `json:"endpoint_id"`
	CampaignID      string            `json:"campaign_id"`
	SourceIP        string            `json:"source_ip"`
	HTTPMethod      string            `json:"http_method"`
	RequestPath     string            `json:"request_path"`
	QueryString     string            `json:"query_string"`
	RequestHeaders  map[string]string `json:"request_headers"`
	ResponseStatus  int               `json:"response_status"`
	ResponseSize    int64             `json:"response_size_bytes"`
	ResponseTimeMs  int               `json:"response_time_ms"`
	TLSVersion      string            `json:"tls_version"`
	LoggedAt        time.Time         `json:"logged_at"`
}

// IngestRequestLogs stores a batch of request logs from an endpoint.
func (s *Service) IngestRequestLogs(ctx context.Context, endpointID string, logs []RequestLogEntry) error {
	if len(logs) == 0 {
		return nil
	}

	// Verify endpoint exists.
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NotFoundError{Msg: "endpoint not found"}
		}
		return fmt.Errorf("ingest request logs: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ingest request logs: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO endpoint_request_logs
			(endpoint_id, campaign_id, source_ip, http_method, request_path, query_string,
			 request_headers, response_status, response_size_bytes, response_time_ms,
			 tls_version, logged_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`)
	if err != nil {
		return fmt.Errorf("ingest request logs: prepare: %w", err)
	}
	defer stmt.Close()

	for _, entry := range logs {
		campaignID := entry.CampaignID
		if campaignID == "" && ep.CampaignID != nil {
			campaignID = *ep.CampaignID
		}

		var campaignIDPtr *string
		if campaignID != "" {
			campaignIDPtr = &campaignID
		}

		loggedAt := entry.LoggedAt
		if loggedAt.IsZero() {
			loggedAt = time.Now().UTC()
		}

		// Encode headers as JSON for JSONB column.
		var headersJSON []byte
		if len(entry.RequestHeaders) > 0 {
			headersJSON, _ = encodeJSON(entry.RequestHeaders)
		}

		_, err := stmt.ExecContext(ctx,
			endpointID, campaignIDPtr, entry.SourceIP, entry.HTTPMethod,
			entry.RequestPath, entry.QueryString, headersJSON,
			entry.ResponseStatus, entry.ResponseSize, entry.ResponseTimeMs,
			entry.TLSVersion, loggedAt,
		)
		if err != nil {
			return fmt.Errorf("ingest request logs: insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ingest request logs: commit: %w", err)
	}

	return nil
}

// RequestLogFilter specifies filters for querying request logs.
type RequestLogFilter struct {
	EndpointID string
	CampaignID string
	SourceIP   string
	Method     string
	Path       string
	Since      time.Time
	Limit      int
	Offset     int
}

// GetRequestLogs returns request logs matching the given filters.
func (s *Service) GetRequestLogs(ctx context.Context, filter RequestLogFilter) ([]RequestLogEntry, int, error) {
	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 100
	}

	// Build query dynamically.
	baseQ := `FROM endpoint_request_logs WHERE 1=1`
	var args []any
	argIdx := 1

	if filter.EndpointID != "" {
		baseQ += fmt.Sprintf(" AND endpoint_id = $%d", argIdx)
		args = append(args, filter.EndpointID)
		argIdx++
	}
	if filter.CampaignID != "" {
		baseQ += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		args = append(args, filter.CampaignID)
		argIdx++
	}
	if filter.SourceIP != "" {
		baseQ += fmt.Sprintf(" AND source_ip = $%d", argIdx)
		args = append(args, filter.SourceIP)
		argIdx++
	}
	if filter.Method != "" {
		baseQ += fmt.Sprintf(" AND http_method = $%d", argIdx)
		args = append(args, filter.Method)
		argIdx++
	}
	if filter.Path != "" {
		baseQ += fmt.Sprintf(" AND request_path LIKE $%d", argIdx)
		args = append(args, "%"+filter.Path+"%")
		argIdx++
	}
	if !filter.Since.IsZero() {
		baseQ += fmt.Sprintf(" AND logged_at >= $%d", argIdx)
		args = append(args, filter.Since)
		argIdx++
	}

	// Count total.
	var total int
	countQ := "SELECT COUNT(*) " + baseQ
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("get request logs: count: %w", err)
	}

	// Fetch page.
	selectQ := fmt.Sprintf(`SELECT endpoint_id, campaign_id, source_ip, http_method, request_path,
		query_string, request_headers, response_status, response_size_bytes, response_time_ms,
		tls_version, logged_at %s ORDER BY logged_at DESC LIMIT $%d OFFSET $%d`,
		baseQ, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, selectQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get request logs: query: %w", err)
	}
	defer rows.Close()

	var results []RequestLogEntry
	for rows.Next() {
		var entry RequestLogEntry
		var campaignID sql.NullString
		var queryString sql.NullString
		var headersJSON []byte
		var tlsVersion sql.NullString

		if err := rows.Scan(
			&entry.EndpointID, &campaignID, &entry.SourceIP, &entry.HTTPMethod,
			&entry.RequestPath, &queryString, &headersJSON,
			&entry.ResponseStatus, &entry.ResponseSize, &entry.ResponseTimeMs,
			&tlsVersion, &entry.LoggedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("get request logs: scan: %w", err)
		}

		if campaignID.Valid {
			entry.CampaignID = campaignID.String
		}
		if queryString.Valid {
			entry.QueryString = queryString.String
		}
		if tlsVersion.Valid {
			entry.TLSVersion = tlsVersion.String
		}
		if len(headersJSON) > 0 {
			_ = decodeJSON(headersJSON, &entry.RequestHeaders)
		}

		results = append(results, entry)
	}

	return results, total, rows.Err()
}

// --- MGMT-11: Error Notifications ---

func (s *Service) notifyEndpointError(ctx context.Context, ep repositories.PhishingEndpoint, errorType, details string) {
	campaignName := "unknown"
	if ep.CampaignID != nil {
		// Try to get campaign name for notification.
		var name sql.NullString
		_ = s.db.QueryRowContext(ctx, `SELECT name FROM campaigns WHERE id = $1`, *ep.CampaignID).Scan(&name)
		if name.Valid {
			campaignName = name.String
		}
	}

	domain := "N/A"
	if ep.Domain != nil {
		domain = *ep.Domain
	}

	actionURL := fmt.Sprintf("/campaigns/%s/endpoint", ep.ID)
	if ep.CampaignID != nil {
		actionURL = fmt.Sprintf("/campaigns/%s/endpoint", *ep.CampaignID)
	}

	s.notifSvc.Create(ctx, notification.CreateNotificationParams{
		Category:     "infrastructure",
		Severity:     "error",
		Title:        fmt.Sprintf("Endpoint Error: %s", errorType),
		Body:         fmt.Sprintf("Campaign: %s\nEndpoint Domain: %s\nError: %s\nDetails: %s", campaignName, domain, errorType, details),
		ResourceType: "phishing_endpoint",
		ResourceID:   ep.ID,
		ActionURL:    actionURL,
		Recipients: notification.RecipientSpec{
			Role: "admin",
		},
	})

	// Also notify the campaign operator if available.
	if ep.CampaignID != nil {
		var createdBy sql.NullString
		_ = s.db.QueryRowContext(ctx, `SELECT created_by FROM campaigns WHERE id = $1`, *ep.CampaignID).Scan(&createdBy)
		if createdBy.Valid {
			s.notifSvc.Create(ctx, notification.CreateNotificationParams{
				Category:     "infrastructure",
				Severity:     "error",
				Title:        fmt.Sprintf("Endpoint Error: %s", errorType),
				Body:         fmt.Sprintf("Campaign: %s\nEndpoint Domain: %s\nError: %s\nDetails: %s", campaignName, domain, errorType, details),
				ResourceType: "phishing_endpoint",
				ResourceID:   ep.ID,
				ActionURL:    actionURL,
				Recipients: notification.RecipientSpec{
					UserIDs: []string{createdBy.String},
				},
			})
		}
	}
}

// RetryEndpoint transitions an endpoint from Error to Configuring for manual retry.
func (s *Service) RetryEndpoint(ctx context.Context, endpointID, actor string) error {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NotFoundError{Msg: "endpoint not found"}
		}
		return fmt.Errorf("retry endpoint: %w", err)
	}

	if ep.State != repositories.EndpointStateError {
		return &ConflictError{Msg: fmt.Sprintf("cannot retry endpoint in %s state, must be in error state", ep.State)}
	}

	_, err = s.sm.Transition(ctx, endpointID, repositories.EndpointStateConfiguring, actor, "manual retry")
	if err != nil {
		return fmt.Errorf("retry endpoint: transition: %w", err)
	}

	resourceType := "phishing_endpoint"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.retried",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
	})

	return nil
}

// --- MGMT-12: TLS Certificate Upload ---

// TLSCertInfo represents an uploaded TLS certificate.
type TLSCertInfo struct {
	ID            string    `json:"id"`
	EndpointID    string    `json:"endpoint_id"`
	Domain        string    `json:"domain"`
	Issuer        string    `json:"issuer"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	Fingerprint   string    `json:"fingerprint_sha256"`
	IsActive      bool      `json:"is_active"`
	UploadedBy    string    `json:"uploaded_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// UploadTLSCertificate validates and stores a manually uploaded TLS certificate.
func (s *Service) UploadTLSCertificate(ctx context.Context, endpointID, actor string, certPEM, keyPEM []byte) (TLSCertInfo, error) {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return TLSCertInfo{}, &NotFoundError{Msg: "endpoint not found"}
		}
		return TLSCertInfo{}, fmt.Errorf("upload tls cert: %w", err)
	}

	// Parse and validate the certificate.
	certInfo, err := parseTLSCertificate(certPEM)
	if err != nil {
		return TLSCertInfo{}, &ValidationError{Msg: fmt.Sprintf("invalid certificate: %s", err)}
	}

	// Validate domain match.
	if ep.Domain != nil && *ep.Domain != "" {
		if !certInfo.matchesDomain(*ep.Domain) {
			return TLSCertInfo{}, &ValidationError{Msg: fmt.Sprintf("certificate does not match endpoint domain %s", *ep.Domain)}
		}
	}

	// Validate not expired and has 24+ hours remaining.
	if time.Now().After(certInfo.notAfter) {
		return TLSCertInfo{}, &ValidationError{Msg: "certificate has expired"}
	}
	if time.Until(certInfo.notAfter) < 24*time.Hour {
		return TLSCertInfo{}, &ValidationError{Msg: "certificate has less than 24 hours remaining validity"}
	}

	// Encrypt certificate and key for storage.
	encCert, err := s.encSvc.Encrypt(certPEM)
	if err != nil {
		return TLSCertInfo{}, fmt.Errorf("upload tls cert: encrypt cert: %w", err)
	}
	encKey, err := s.encSvc.Encrypt(keyPEM)
	if err != nil {
		return TLSCertInfo{}, fmt.Errorf("upload tls cert: encrypt key: %w", err)
	}

	// Deactivate previous active certificate.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE endpoint_tls_certificates SET is_active = false, replaced_at = now() WHERE endpoint_id = $1 AND is_active = true`,
		endpointID)

	domain := ""
	if ep.Domain != nil {
		domain = *ep.Domain
	}

	// Insert the new certificate.
	var id string
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO endpoint_tls_certificates
			(endpoint_id, domain, cert_pem_encrypted, key_pem_encrypted, issuer,
			 not_before, not_after, fingerprint_sha256, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		endpointID, domain, encCert, encKey, certInfo.issuer,
		certInfo.notBefore, certInfo.notAfter, certInfo.fingerprint, actor,
	).Scan(&id)
	if err != nil {
		return TLSCertInfo{}, fmt.Errorf("upload tls cert: insert: %w", err)
	}

	resourceType := "phishing_endpoint"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.tls_certificate_uploaded",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
		Details: map[string]any{
			"domain":      domain,
			"fingerprint": certInfo.fingerprint,
			"not_after":   certInfo.notAfter.Format(time.RFC3339),
		},
	})

	return TLSCertInfo{
		ID:          id,
		EndpointID:  endpointID,
		Domain:      domain,
		Issuer:      certInfo.issuer,
		NotBefore:   certInfo.notBefore,
		NotAfter:    certInfo.notAfter,
		Fingerprint: certInfo.fingerprint,
		IsActive:    true,
		UploadedBy:  actor,
	}, nil
}

// --- MGMT-13: Phishing Report Webhook ---

// PhishingReportPayload represents an incoming phishing report from external systems.
type PhishingReportPayload struct {
	ReporterEmail string `json:"reporter_email"`
	MessageID     string `json:"message_id"`
	SubjectLine   string `json:"subject_line"`
}

// PhishingReport represents a stored phishing report.
type PhishingReport struct {
	ID            string    `json:"id"`
	CampaignID    *string   `json:"campaign_id,omitempty"`
	TargetID      *string   `json:"target_id,omitempty"`
	ReporterEmail string    `json:"reporter_email"`
	MessageID     string    `json:"message_id,omitempty"`
	SubjectLine   string    `json:"subject_line,omitempty"`
	Matched       bool      `json:"matched"`
	Source        string    `json:"source"`
	ReportedBy    *string   `json:"reported_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ProcessPhishingReport handles an incoming phishing report (webhook or manual).
func (s *Service) ProcessPhishingReport(ctx context.Context, payload PhishingReportPayload, source, actor string) (PhishingReport, error) {
	if payload.ReporterEmail == "" {
		return PhishingReport{}, &ValidationError{Msg: "reporter_email is required"}
	}
	if payload.MessageID == "" && payload.SubjectLine == "" {
		return PhishingReport{}, &ValidationError{Msg: "either message_id or subject_line is required"}
	}

	report := PhishingReport{
		ReporterEmail: payload.ReporterEmail,
		MessageID:     payload.MessageID,
		SubjectLine:   payload.SubjectLine,
		Source:        source,
	}

	if actor != "" {
		report.ReportedBy = &actor
	}

	// Try to match to a campaign and target.
	if payload.MessageID != "" {
		var campaignID, targetID sql.NullString
		// Match via message_id in campaign target events.
		err := s.db.QueryRowContext(ctx, `
			SELECT cte.campaign_id, cte.target_id
			FROM campaign_target_events cte
			WHERE cte.event_data->>'message_id' = $1
			LIMIT 1`, payload.MessageID).Scan(&campaignID, &targetID)
		if err == nil {
			report.Matched = true
			if campaignID.Valid {
				report.CampaignID = &campaignID.String
			}
			if targetID.Valid {
				report.TargetID = &targetID.String
			}
		}
	}

	// Insert the report.
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO phishing_reports
			(campaign_id, target_id, reporter_email, message_id, subject_line, matched, source, reported_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		report.CampaignID, report.TargetID, report.ReporterEmail,
		report.MessageID, report.SubjectLine, report.Matched, report.Source, report.ReportedBy,
	).Scan(&id)
	if err != nil {
		return PhishingReport{}, fmt.Errorf("process phishing report: insert: %w", err)
	}
	report.ID = id

	// Audit log.
	resourceType := "phishing_report"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "phishing_report.received",
		ResourceType: &resourceType,
		ResourceID:   &id,
		Details: map[string]any{
			"source":         source,
			"reporter_email": payload.ReporterEmail,
			"matched":        report.Matched,
		},
	})

	return report, nil
}

// --- MGMT-14: Endpoint Status/Details ---

// EndpointDTO is the API response for an endpoint.
type EndpointDTO struct {
	ID              string     `json:"id"`
	CampaignID      *string    `json:"campaign_id,omitempty"`
	CloudProvider   string     `json:"cloud_provider"`
	Region          string     `json:"region"`
	InstanceID      *string    `json:"instance_id,omitempty"`
	PublicIP        *string    `json:"public_ip,omitempty"`
	Domain          *string    `json:"domain,omitempty"`
	State           string     `json:"state"`
	BinaryHash      *string    `json:"binary_hash,omitempty"`
	ControlPort     *int       `json:"control_port,omitempty"`
	SSHKeyID        *string    `json:"ssh_key_id,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	ProvisionedAt   *time.Time `json:"provisioned_at,omitempty"`
	ActivatedAt     *time.Time `json:"activated_at,omitempty"`
	TerminatedAt    *time.Time `json:"terminated_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ToDTO converts a repository model to an API DTO.
func ToDTO(ep repositories.PhishingEndpoint) EndpointDTO {
	return EndpointDTO{
		ID:              ep.ID,
		CampaignID:      ep.CampaignID,
		CloudProvider:   string(ep.CloudProvider),
		Region:          ep.Region,
		InstanceID:      ep.InstanceID,
		PublicIP:        ep.PublicIP,
		Domain:          ep.Domain,
		State:           string(ep.State),
		BinaryHash:      ep.BinaryHash,
		ControlPort:     ep.ControlPort,
		SSHKeyID:        ep.SSHKeyID,
		ErrorMessage:    ep.ErrorMessage,
		LastHeartbeatAt: ep.LastHeartbeatAt,
		ProvisionedAt:   ep.ProvisionedAt,
		ActivatedAt:     ep.ActivatedAt,
		TerminatedAt:    ep.TerminatedAt,
		CreatedAt:       ep.CreatedAt,
		UpdatedAt:       ep.UpdatedAt,
	}
}

// GetEndpoint returns a single endpoint by ID.
func (s *Service) GetEndpoint(ctx context.Context, endpointID string) (EndpointDTO, error) {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return EndpointDTO{}, &NotFoundError{Msg: "endpoint not found"}
		}
		return EndpointDTO{}, fmt.Errorf("get endpoint: %w", err)
	}
	return ToDTO(ep), nil
}

// GetCampaignEndpoint returns the endpoint for a campaign.
func (s *Service) GetCampaignEndpoint(ctx context.Context, campaignID string) (EndpointDTO, error) {
	eps, err := s.repo.ListByCampaign(ctx, campaignID)
	if err != nil {
		return EndpointDTO{}, fmt.Errorf("get campaign endpoint: %w", err)
	}
	if len(eps) == 0 {
		return EndpointDTO{}, &NotFoundError{Msg: "no endpoint found for campaign"}
	}
	// Return the most recent non-terminated endpoint, or the latest if all terminated.
	for i := len(eps) - 1; i >= 0; i-- {
		if eps[i].State != repositories.EndpointStateTerminated {
			return ToDTO(eps[i]), nil
		}
	}
	return ToDTO(eps[len(eps)-1]), nil
}

// GetEndpointSSHKey retrieves an SSH key by ID. Returns a NotFoundError if not found.
func (s *Service) GetEndpointSSHKey(ctx context.Context, keyID string) (repositories.EndpointSSHKey, error) {
	key, err := s.repo.GetSSHKey(ctx, keyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return repositories.EndpointSSHKey{}, &NotFoundError{Msg: "SSH key not found"}
		}
		return repositories.EndpointSSHKey{}, fmt.Errorf("get ssh key: %w", err)
	}
	return key, nil
}

// GetEndpointTransitions returns the state transition audit history for an endpoint.
func (s *Service) GetEndpointTransitions(ctx context.Context, endpointID string) ([]repositories.EndpointStateTransition, error) {
	return s.repo.GetTransitions(ctx, endpointID)
}

// CheckIPChange detects when an endpoint's public IP changes and sends a notification
// warning that SPF records may need updating for the associated domain.
func (s *Service) CheckIPChange(ctx context.Context, endpointID, oldIP, newIP string) {
	if oldIP == newIP || oldIP == "" || newIP == "" {
		return
	}

	slog.Warn("endpoint IP changed", "endpoint_id", endpointID, "old_ip", oldIP, "new_ip", newIP)

	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		slog.Error("check ip change: get endpoint", "error", err)
		return
	}

	domain := ""
	if ep.Domain != nil {
		domain = *ep.Domain
	}

	msg := fmt.Sprintf(
		"Endpoint %s IP changed from %s to %s. SPF records for domain %q may need updating.",
		endpointID, oldIP, newIP, domain,
	)

	s.notifSvc.Create(ctx, notification.CreateNotificationParams{
		Category:     "infrastructure",
		Severity:     "warning",
		Title:        "Endpoint IP Changed",
		Body:         msg,
		ResourceType: "phishing_endpoint",
		ResourceID:   endpointID,
		Recipients:   notification.RecipientSpec{Role: "admin"},
	})

	// Broadcast to WebSocket clients.
	s.broadcastEndpointEvent("endpoint_ip_changed", map[string]any{
		"endpoint_id": endpointID,
		"old_ip":      oldIP,
		"new_ip":      newIP,
		"domain":      domain,
	})
}
