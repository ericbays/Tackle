package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EndpointState represents the lifecycle state of a phishing endpoint.
type EndpointState string

const (
	// EndpointStateRequested means the endpoint has been requested but not yet provisioned.
	EndpointStateRequested EndpointState = "requested"
	// EndpointStateProvisioning means the cloud VM is being created.
	EndpointStateProvisioning EndpointState = "provisioning"
	// EndpointStateConfiguring means the endpoint is being configured (binary deployment, TLS, etc.).
	EndpointStateConfiguring EndpointState = "configuring"
	// EndpointStateActive means the endpoint is live and serving traffic.
	EndpointStateActive EndpointState = "active"
	// EndpointStateStopped means the endpoint is stopped but can be restarted.
	EndpointStateStopped EndpointState = "stopped"
	// EndpointStateError means the endpoint encountered an error.
	EndpointStateError EndpointState = "error"
	// EndpointStateTerminated means the endpoint has been destroyed.
	EndpointStateTerminated EndpointState = "terminated"
)

// PhishingEndpoint is the DB model for a phishing_endpoints row.
type PhishingEndpoint struct {
	ID                string
	CampaignID        *string
	CloudProvider     CloudProviderType
	Region            string
	InstanceID        *string
	PublicIP          *string
	Domain            *string
	State             EndpointState
	BinaryHash        *string
	ControlPort       *int
	AuthToken         []byte
	SSHKeyID          *string
	ErrorMessage      *string
	InstanceType      *string
	TLSCertInfo       []byte // JSONB
	CloudCredentialID *string
	LastHeartbeatAt   *time.Time
	ProvisionedAt     *time.Time
	ActivatedAt       *time.Time
	TerminatedAt      *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// EndpointStateTransition is the DB model for an endpoint_state_transitions row.
type EndpointStateTransition struct {
	ID         string
	EndpointID string
	FromState  EndpointState
	ToState    EndpointState
	Actor      string
	Reason     string
	CreatedAt  time.Time
}

// EndpointSSHKey is the DB model for an endpoint_ssh_keys row.
type EndpointSSHKey struct {
	ID                  string
	CampaignID          string
	PublicKey           string
	PrivateKeyEncrypted []byte
	CreatedAt           time.Time
	DestroyedAt         *time.Time
}

// ProxmoxIPAllocation is the DB model for a proxmox_ip_allocations row.
type ProxmoxIPAllocation struct {
	ID                string
	CloudCredentialID string
	IPAddress         string
	EndpointID        *string
	AllocatedAt       *time.Time
	ReleasedAt        *time.Time
	CreatedAt         time.Time
}

// PhishingEndpointRepository provides database operations for phishing endpoints.
type PhishingEndpointRepository struct {
	db *sql.DB
}

// NewPhishingEndpointRepository creates a new PhishingEndpointRepository.
func NewPhishingEndpointRepository(db *sql.DB) *PhishingEndpointRepository {
	return &PhishingEndpointRepository{db: db}
}

// Create inserts a new phishing endpoint and returns the created row.
func (r *PhishingEndpointRepository) Create(ctx context.Context, ep PhishingEndpoint) (PhishingEndpoint, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO phishing_endpoints
			(id, campaign_id, cloud_provider, region, state)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, campaign_id, cloud_provider, region, instance_id, public_ip, domain,
		          state, binary_hash, control_port, auth_token, ssh_key_id, error_message,
		          instance_type, tls_cert_info, cloud_credential_id, last_heartbeat_at, provisioned_at, activated_at, terminated_at,
		          created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, ep.CampaignID, string(ep.CloudProvider), ep.Region, string(EndpointStateRequested),
	))
}

// GetByID retrieves a phishing endpoint by UUID. Returns sql.ErrNoRows if not found.
func (r *PhishingEndpointRepository) GetByID(ctx context.Context, id string) (PhishingEndpoint, error) {
	const q = `
		SELECT id, campaign_id, cloud_provider, region, instance_id, public_ip, domain,
		       state, binary_hash, control_port, auth_token, ssh_key_id, error_message,
		       instance_type, tls_cert_info, cloud_credential_id, last_heartbeat_at, provisioned_at, activated_at, terminated_at,
		       created_at, updated_at
		FROM phishing_endpoints WHERE id = $1`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// ListByCampaign returns all endpoints for a given campaign.
func (r *PhishingEndpointRepository) ListByCampaign(ctx context.Context, campaignID string) ([]PhishingEndpoint, error) {
	const q = `
		SELECT id, campaign_id, cloud_provider, region, instance_id, public_ip, domain,
		       state, binary_hash, control_port, auth_token, ssh_key_id, error_message,
		       instance_type, tls_cert_info, cloud_credential_id, last_heartbeat_at, provisioned_at, activated_at, terminated_at,
		       created_at, updated_at
		FROM phishing_endpoints WHERE campaign_id = $1
		ORDER BY created_at ASC`
	return r.scanMany(ctx, q, campaignID)
}

// ListByState returns all endpoints in a given state.
func (r *PhishingEndpointRepository) ListByState(ctx context.Context, state EndpointState) ([]PhishingEndpoint, error) {
	const q = `
		SELECT id, campaign_id, cloud_provider, region, instance_id, public_ip, domain,
		       state, binary_hash, control_port, auth_token, ssh_key_id, error_message,
		       instance_type, tls_cert_info, cloud_credential_id, last_heartbeat_at, provisioned_at, activated_at, terminated_at,
		       created_at, updated_at
		FROM phishing_endpoints WHERE state = $1
		ORDER BY created_at ASC`
	return r.scanMany(ctx, q, string(state))
}

// UpdateState atomically transitions an endpoint to a new state and records the transition.
// Returns the updated endpoint. Uses a transaction to ensure atomicity.
func (r *PhishingEndpointRepository) UpdateState(ctx context.Context, id string, fromState, toState EndpointState, actor, reason string) (PhishingEndpoint, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return PhishingEndpoint{}, fmt.Errorf("endpoint: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update the endpoint state with optimistic locking on current state.
	var timestampClause string
	switch toState {
	case EndpointStateProvisioning:
		timestampClause = ", provisioned_at = now()"
	case EndpointStateActive:
		timestampClause = ", activated_at = now()"
	case EndpointStateTerminated:
		timestampClause = ", terminated_at = now()"
	case EndpointStateError:
		timestampClause = ", error_message = $4"
	}

	var updateQ string
	var args []any
	if toState == EndpointStateError {
		updateQ = fmt.Sprintf(`
			UPDATE phishing_endpoints SET state = $1%s
			WHERE id = $2 AND state = $3
			RETURNING id, campaign_id, cloud_provider, region, instance_id, public_ip, domain,
			          state, binary_hash, control_port, auth_token, ssh_key_id, error_message,
			          last_heartbeat_at, provisioned_at, activated_at, terminated_at,
			          created_at, updated_at`, timestampClause)
		args = []any{string(toState), id, string(fromState), reason}
	} else {
		updateQ = fmt.Sprintf(`
			UPDATE phishing_endpoints SET state = $1%s
			WHERE id = $2 AND state = $3
			RETURNING id, campaign_id, cloud_provider, region, instance_id, public_ip, domain,
			          state, binary_hash, control_port, auth_token, ssh_key_id, error_message,
			          last_heartbeat_at, provisioned_at, activated_at, terminated_at,
			          created_at, updated_at`, timestampClause)
		args = []any{string(toState), id, string(fromState)}
	}

	ep, err := r.scanOneRow(tx.QueryRowContext(ctx, updateQ, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return PhishingEndpoint{}, fmt.Errorf("endpoint: state transition failed: endpoint %s not in state %s", id, fromState)
		}
		return PhishingEndpoint{}, fmt.Errorf("endpoint: update state: %w", err)
	}

	// Record the state transition.
	transitionID := uuid.New().String()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO endpoint_state_transitions (id, endpoint_id, from_state, to_state, actor, reason) VALUES ($1, $2, $3, $4, $5, $6)`,
		transitionID, id, string(fromState), string(toState), actor, reason,
	)
	if err != nil {
		return PhishingEndpoint{}, fmt.Errorf("endpoint: record transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return PhishingEndpoint{}, fmt.Errorf("endpoint: commit: %w", err)
	}

	return ep, nil
}

// UpdateInstanceInfo updates the instance_id and public_ip after cloud provisioning.
func (r *PhishingEndpointRepository) UpdateInstanceInfo(ctx context.Context, id, instanceID, publicIP string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET instance_id = $1, public_ip = $2 WHERE id = $3`,
		instanceID, publicIP, id)
	if err != nil {
		return fmt.Errorf("endpoint: update instance info: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("endpoint: update instance info: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateDomain updates the domain associated with an endpoint.
func (r *PhishingEndpointRepository) UpdateDomain(ctx context.Context, id, domain string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET domain = $1 WHERE id = $2`,
		domain, id)
	if err != nil {
		return fmt.Errorf("endpoint: update domain: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("endpoint: update domain: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateBinaryHash updates the binary hash and SSH key reference after configuration.
func (r *PhishingEndpointRepository) UpdateBinaryHash(ctx context.Context, id, binaryHash string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET binary_hash = $1 WHERE id = $2`,
		binaryHash, id)
	if err != nil {
		return fmt.Errorf("endpoint: update binary hash: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("endpoint: update binary hash: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateControlInfo updates the control port and auth token.
func (r *PhishingEndpointRepository) UpdateControlInfo(ctx context.Context, id string, controlPort int, authToken []byte) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET control_port = $1, auth_token = $2 WHERE id = $3`,
		controlPort, authToken, id)
	if err != nil {
		return fmt.Errorf("endpoint: update control info: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("endpoint: update control info: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateSSHKeyID associates an SSH key with an endpoint.
func (r *PhishingEndpointRepository) UpdateSSHKeyID(ctx context.Context, id, sshKeyID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET ssh_key_id = $1 WHERE id = $2`,
		sshKeyID, id)
	if err != nil {
		return fmt.Errorf("endpoint: update ssh key id: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("endpoint: update ssh key id: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateIPAllocationID stores the cloud provider's IP allocation ID for proper release on termination.
// Stored in the instance_type column as "alloc:{id}" to distinguish from actual instance types.
func (r *PhishingEndpointRepository) UpdateIPAllocationID(ctx context.Context, id, allocationID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET instance_type = $1 WHERE id = $2`,
		"alloc:"+allocationID, id)
	if err != nil {
		return fmt.Errorf("endpoint: update ip allocation id: %w", err)
	}
	return nil
}

// GetIPAllocationID retrieves the stored IP allocation ID for an endpoint. Returns empty string if not set.
func (r *PhishingEndpointRepository) GetIPAllocationID(ctx context.Context, id string) (string, error) {
	var instanceType *string
	err := r.db.QueryRowContext(ctx,
		`SELECT instance_type FROM phishing_endpoints WHERE id = $1`, id).Scan(&instanceType)
	if err != nil {
		return "", err
	}
	if instanceType == nil {
		return "", nil
	}
	const prefix = "alloc:"
	if len(*instanceType) > len(prefix) && (*instanceType)[:len(prefix)] == prefix {
		return (*instanceType)[len(prefix):], nil
	}
	return "", nil
}

// UpdateTLSCertInfo stores TLS certificate metadata as JSONB on the endpoint.
func (r *PhishingEndpointRepository) UpdateTLSCertInfo(ctx context.Context, id string, certInfoJSON []byte) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET tls_cert_info = $1 WHERE id = $2`, certInfoJSON, id)
	if err != nil {
		return fmt.Errorf("endpoint: update tls cert info: %w", err)
	}
	return nil
}

// UpdateHeartbeat updates the last_heartbeat_at timestamp.
func (r *PhishingEndpointRepository) UpdateHeartbeat(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE phishing_endpoints SET last_heartbeat_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("endpoint: update heartbeat: %w", err)
	}
	return nil
}

// GetTransitions returns the state transition history for an endpoint.
func (r *PhishingEndpointRepository) GetTransitions(ctx context.Context, endpointID string) ([]EndpointStateTransition, error) {
	const q = `
		SELECT id, endpoint_id, from_state, to_state, actor, reason, created_at
		FROM endpoint_state_transitions WHERE endpoint_id = $1
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, q, endpointID)
	if err != nil {
		return nil, fmt.Errorf("endpoint: get transitions: %w", err)
	}
	defer rows.Close()

	var results []EndpointStateTransition
	for rows.Next() {
		var t EndpointStateTransition
		if err := rows.Scan(&t.ID, &t.EndpointID, &t.FromState, &t.ToState, &t.Actor, &t.Reason, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("endpoint: scan transition: %w", err)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

// CreateSSHKey inserts a new SSH key pair for a campaign.
func (r *PhishingEndpointRepository) CreateSSHKey(ctx context.Context, key EndpointSSHKey) (EndpointSSHKey, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO endpoint_ssh_keys (id, campaign_id, public_key, private_key_encrypted)
		VALUES ($1, $2, $3, $4)
		RETURNING id, campaign_id, public_key, private_key_encrypted, created_at, destroyed_at`
	row := r.db.QueryRowContext(ctx, q, id, key.CampaignID, key.PublicKey, key.PrivateKeyEncrypted)
	var k EndpointSSHKey
	err := row.Scan(&k.ID, &k.CampaignID, &k.PublicKey, &k.PrivateKeyEncrypted, &k.CreatedAt, &k.DestroyedAt)
	if err != nil {
		return EndpointSSHKey{}, fmt.Errorf("endpoint: create ssh key: %w", err)
	}
	return k, nil
}

// DestroySSHKey marks an SSH key as destroyed.
// GetSSHKey retrieves an SSH key by ID. Returns sql.ErrNoRows if not found.
func (r *PhishingEndpointRepository) GetSSHKey(ctx context.Context, id string) (EndpointSSHKey, error) {
	const q = `
		SELECT id, campaign_id, public_key, private_key_encrypted, created_at, destroyed_at
		FROM endpoint_ssh_keys WHERE id = $1`
	var key EndpointSSHKey
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&key.ID, &key.CampaignID, &key.PublicKey, &key.PrivateKeyEncrypted,
		&key.CreatedAt, &key.DestroyedAt,
	)
	if err != nil {
		return EndpointSSHKey{}, err
	}
	return key, nil
}

func (r *PhishingEndpointRepository) DestroySSHKey(ctx context.Context, id string) error {
	// Overwrite private key with zeros (defense in depth) and mark as destroyed.
	result, err := r.db.ExecContext(ctx,
		`UPDATE endpoint_ssh_keys SET private_key_encrypted = '\x00', destroyed_at = now() WHERE id = $1 AND destroyed_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("endpoint: destroy ssh key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("endpoint: ssh key not found or already destroyed")
	}
	return nil
}

// ListSSHKeysByCampaign returns all non-destroyed SSH keys for a campaign.
func (r *PhishingEndpointRepository) ListSSHKeysByCampaign(ctx context.Context, campaignID string) ([]EndpointSSHKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, public_key, private_key_encrypted, created_at, destroyed_at
		 FROM endpoint_ssh_keys WHERE campaign_id = $1 AND destroyed_at IS NULL`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("endpoint: list ssh keys by campaign: %w", err)
	}
	defer rows.Close()

	var result []EndpointSSHKey
	for rows.Next() {
		var k EndpointSSHKey
		if err := rows.Scan(&k.ID, &k.CampaignID, &k.PublicKey, &k.PrivateKeyEncrypted, &k.CreatedAt, &k.DestroyedAt); err != nil {
			return nil, fmt.Errorf("endpoint: scan ssh key: %w", err)
		}
		result = append(result, k)
	}
	if result == nil {
		result = []EndpointSSHKey{}
	}
	return result, nil
}

// AllocateProxmoxIP atomically allocates the next available IP from a credential's pool.
func (r *PhishingEndpointRepository) AllocateProxmoxIP(ctx context.Context, credentialID, endpointID string) (ProxmoxIPAllocation, error) {
	// Find the first unreleased allocation without an endpoint (pre-populated pool)
	// or insert a new allocation if using dynamic allocation.
	// For now, we use the pool-based approach: find next unallocated IP.
	const q = `
		UPDATE proxmox_ip_allocations
		SET endpoint_id = $1, allocated_at = now()
		WHERE id = (
			SELECT id FROM proxmox_ip_allocations
			WHERE cloud_credential_id = $2
			  AND endpoint_id IS NULL
			  AND released_at IS NULL
			ORDER BY ip_address ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, cloud_credential_id, ip_address, endpoint_id, allocated_at, released_at, created_at`
	var a ProxmoxIPAllocation
	err := r.db.QueryRowContext(ctx, q, endpointID, credentialID).Scan(
		&a.ID, &a.CloudCredentialID, &a.IPAddress, &a.EndpointID,
		&a.AllocatedAt, &a.ReleasedAt, &a.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ProxmoxIPAllocation{}, fmt.Errorf("endpoint: no available IPs in pool for credential %s", credentialID)
		}
		return ProxmoxIPAllocation{}, fmt.Errorf("endpoint: allocate proxmox ip: %w", err)
	}
	return a, nil
}

// ReleaseProxmoxIP releases an allocated IP back to the pool.
func (r *PhishingEndpointRepository) ReleaseProxmoxIP(ctx context.Context, endpointID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE proxmox_ip_allocations SET endpoint_id = NULL, released_at = now() WHERE endpoint_id = $1 AND released_at IS NULL`,
		endpointID)
	if err != nil {
		return fmt.Errorf("endpoint: release proxmox ip: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil // No IP was allocated — not an error.
	}
	return nil
}

// PopulateProxmoxIPPool inserts available IPs into the pool for a credential.
func (r *PhishingEndpointRepository) PopulateProxmoxIPPool(ctx context.Context, credentialID string, ips []string) error {
	for _, ip := range ips {
		id := uuid.New().String()
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO proxmox_ip_allocations (id, cloud_credential_id, ip_address) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING`,
			id, credentialID, ip)
		if err != nil {
			return fmt.Errorf("endpoint: populate ip pool: %w", err)
		}
	}
	return nil
}

// GetProxmoxPoolUtilization returns total and allocated counts for a credential's IP pool.
func (r *PhishingEndpointRepository) GetProxmoxPoolUtilization(ctx context.Context, credentialID string) (total, allocated int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(endpoint_id) FROM proxmox_ip_allocations WHERE cloud_credential_id = $1 AND released_at IS NULL`,
		credentialID).Scan(&total, &allocated)
	if err != nil {
		return 0, 0, fmt.Errorf("endpoint: pool utilization: %w", err)
	}
	return total, allocated, nil
}

func (r *PhishingEndpointRepository) scanOne(row *sql.Row) (PhishingEndpoint, error) {
	return r.scanOneRow(row)
}

func (r *PhishingEndpointRepository) scanOneRow(row *sql.Row) (PhishingEndpoint, error) {
	var ep PhishingEndpoint
	err := row.Scan(
		&ep.ID, &ep.CampaignID, &ep.CloudProvider, &ep.Region, &ep.InstanceID,
		&ep.PublicIP, &ep.Domain, &ep.State, &ep.BinaryHash, &ep.ControlPort,
		&ep.AuthToken, &ep.SSHKeyID, &ep.ErrorMessage,
		&ep.InstanceType, &ep.TLSCertInfo, &ep.CloudCredentialID, &ep.LastHeartbeatAt, &ep.ProvisionedAt, &ep.ActivatedAt, &ep.TerminatedAt,
		&ep.CreatedAt, &ep.UpdatedAt,
	)
	if err != nil {
		return PhishingEndpoint{}, fmt.Errorf("endpoint: scan: %w", err)
	}
	return ep, nil
}

func (r *PhishingEndpointRepository) scanMany(ctx context.Context, query string, args ...any) ([]PhishingEndpoint, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("endpoint: query: %w", err)
	}
	defer rows.Close()

	var results []PhishingEndpoint
	for rows.Next() {
		var ep PhishingEndpoint
		if err := rows.Scan(
			&ep.ID, &ep.CampaignID, &ep.CloudProvider, &ep.Region, &ep.InstanceID,
			&ep.PublicIP, &ep.Domain, &ep.State, &ep.BinaryHash, &ep.ControlPort,
			&ep.AuthToken, &ep.SSHKeyID, &ep.ErrorMessage,
			&ep.LastHeartbeatAt, &ep.ProvisionedAt, &ep.ActivatedAt, &ep.TerminatedAt,
			&ep.CreatedAt, &ep.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("endpoint: scan row: %w", err)
		}
		results = append(results, ep)
	}
	return results, rows.Err()
}
