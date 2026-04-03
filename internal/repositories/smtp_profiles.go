package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SMTPAuthType is the authentication method used for an SMTP connection.
type SMTPAuthType string

const (
	// SMTPAuthNone sends no AUTH command.
	SMTPAuthNone SMTPAuthType = "none"
	// SMTPAuthPlain uses AUTH PLAIN.
	SMTPAuthPlain SMTPAuthType = "plain"
	// SMTPAuthLogin uses AUTH LOGIN.
	SMTPAuthLogin SMTPAuthType = "login"
	// SMTPAuthCRAMMD5 uses AUTH CRAM-MD5.
	SMTPAuthCRAMMD5 SMTPAuthType = "cram_md5"
	// SMTPAuthXOAUTH2 uses AUTH XOAUTH2.
	SMTPAuthXOAUTH2 SMTPAuthType = "xoauth2"
)

// SMTPTLSMode is the TLS negotiation mode for an SMTP connection.
type SMTPTLSMode string

const (
	// SMTPTLSNone sends no TLS (plaintext).
	SMTPTLSNone SMTPTLSMode = "none"
	// SMTPTLSStartTLS issues a STARTTLS command after the initial greeting.
	SMTPTLSStartTLS SMTPTLSMode = "starttls"
	// SMTPTLSTLS wraps the connection in TLS immediately (implicit TLS, port 465).
	SMTPTLSTLS SMTPTLSMode = "tls"
)

// SMTPProfileStatus is the last-known connection test result for an SMTP profile.
type SMTPProfileStatus string

const (
	// SMTPStatusUntested means no test has been performed yet.
	SMTPStatusUntested SMTPProfileStatus = "untested"
	// SMTPStatusHealthy means the last test succeeded.
	SMTPStatusHealthy SMTPProfileStatus = "healthy"
	// SMTPStatusError means the last test failed.
	SMTPStatusError SMTPProfileStatus = "error"
)

// SMTPProfile is the DB model for an smtp_profiles row.
type SMTPProfile struct {
	ID                 string
	Name               string
	Description        *string
	Host               string
	Port               int
	AuthType           SMTPAuthType
	UsernameEncrypted  []byte
	PasswordEncrypted  []byte
	TLSMode            SMTPTLSMode
	TLSSkipVerify      bool
	FromAddress        string
	FromName           *string
	ReplyTo            *string
	CustomHELO         *string
	MaxSendRate        *int
	MaxConnections     int
	TimeoutConnect     int
	TimeoutSend        int
	Status             SMTPProfileStatus
	StatusMessage      *string
	LastTestedAt       *time.Time
	CreatedBy          string
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SMTPProfileFilters controls optional filtering for List.
type SMTPProfileFilters struct {
	Status     string // empty = all
	NameSearch string // empty = all
}

// SMTPProfileUpdate holds mutable fields for an update operation.
type SMTPProfileUpdate struct {
	Name               *string
	Description        *string
	Host               *string
	Port               *int
	AuthType           *SMTPAuthType
	UsernameEncrypted  []byte  // nil = no change
	PasswordEncrypted  []byte  // nil = no change
	TLSMode            *SMTPTLSMode
	TLSSkipVerify      *bool
	FromAddress        *string
	FromName           *string
	ReplyTo            *string
	CustomHELO         *string
	MaxSendRate        *int
	MaxConnections     *int
	TimeoutConnect     *int
	TimeoutSend        *int
}

// SMTPCampaignAssoc is a minimal view of a campaign-SMTP association for deletion protection.
type SMTPCampaignAssoc struct {
	CampaignID    string
	SMTPProfileID string
}

// CampaignSMTPProfile is the DB model for a campaign_smtp_profiles row.
type CampaignSMTPProfile struct {
	ID                  string
	CampaignID          string
	SMTPProfileID       string
	Priority            int
	Weight              int
	FromAddressOverride *string
	FromNameOverride    *string
	ReplyToOverride     *string
	SegmentFilter       []byte // raw JSONB
	CreatedAt           time.Time
}

// CampaignSendSchedule is the DB model for a campaign_send_schedules row.
type CampaignSendSchedule struct {
	ID                 string
	CampaignID         string
	SendingStrategy    string
	SendWindowStart    *string // HH:MM:SS
	SendWindowEnd      *string // HH:MM:SS
	SendWindowTimezone *string
	SendWindowDays     []int
	CampaignRateLimit  *int
	MinDelayMs         int
	MaxDelayMs         int
	BatchSize          *int
	BatchPauseSeconds  *int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SMTPProfileRepository provides database operations for smtp_profiles.
type SMTPProfileRepository struct {
	db *sql.DB
}

// NewSMTPProfileRepository creates a new SMTPProfileRepository.
func NewSMTPProfileRepository(db *sql.DB) *SMTPProfileRepository {
	return &SMTPProfileRepository{db: db}
}

// Create inserts a new SMTP profile and returns the created row.
func (r *SMTPProfileRepository) Create(ctx context.Context, p SMTPProfile) (SMTPProfile, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO smtp_profiles
			(id, name, description, host, port, auth_type,
			 username_encrypted, password_encrypted,
			 tls_mode, tls_skip_verify,
			 from_address, from_name, reply_to, custom_helo,
			 max_send_rate, max_connections, timeout_connect, timeout_send,
			 created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id, name, description, host, port, auth_type,
		          username_encrypted, password_encrypted,
		          tls_mode, tls_skip_verify,
		          from_address, from_name, reply_to, custom_helo,
		          max_send_rate, max_connections, timeout_connect, timeout_send,
		          status, status_message, last_tested_at, created_by, deleted_at, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, p.Name, p.Description, p.Host, p.Port, string(p.AuthType),
		nullBytes(p.UsernameEncrypted), nullBytes(p.PasswordEncrypted),
		string(p.TLSMode), p.TLSSkipVerify,
		p.FromAddress, p.FromName, p.ReplyTo, p.CustomHELO,
		p.MaxSendRate, p.MaxConnections, p.TimeoutConnect, p.TimeoutSend,
		p.CreatedBy,
	))
}

// GetByID retrieves an SMTP profile by UUID. Returns sql.ErrNoRows if not found.
func (r *SMTPProfileRepository) GetByID(ctx context.Context, id string) (SMTPProfile, error) {
	const q = `
		SELECT id, name, description, host, port, auth_type,
		       username_encrypted, password_encrypted,
		       tls_mode, tls_skip_verify,
		       from_address, from_name, reply_to, custom_helo,
		       max_send_rate, max_connections, timeout_connect, timeout_send,
		       status, status_message, last_tested_at, created_by, deleted_at, created_at, updated_at
		FROM smtp_profiles WHERE id = $1 AND deleted_at IS NULL`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// List returns SMTP profiles with optional filtering and name search.
func (r *SMTPProfileRepository) List(ctx context.Context, filters SMTPProfileFilters) ([]SMTPProfile, error) {
	args := []any{}
	argIdx := 1
	where := "WHERE deleted_at IS NULL"

	if filters.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.NameSearch != "" {
		where += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.NameSearch+"%")
		argIdx++
	}
	_ = argIdx

	q := fmt.Sprintf(`
		SELECT id, name, description, host, port, auth_type,
		       username_encrypted, password_encrypted,
		       tls_mode, tls_skip_verify,
		       from_address, from_name, reply_to, custom_helo,
		       max_send_rate, max_connections, timeout_connect, timeout_send,
		       status, status_message, last_tested_at, created_by, deleted_at, created_at, updated_at
		FROM smtp_profiles %s ORDER BY name ASC`, where)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("smtp profiles: list: %w", err)
	}
	defer rows.Close()

	var results []SMTPProfile
	for rows.Next() {
		p, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("smtp profiles: list scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("smtp profiles: list rows: %w", err)
	}
	return results, nil
}

// Update applies changes to the SMTP profile identified by id.
func (r *SMTPProfileRepository) Update(ctx context.Context, id string, upd SMTPProfileUpdate) (SMTPProfile, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if upd.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *upd.Name)
		argIdx++
	}
	if upd.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *upd.Description)
		argIdx++
	}
	if upd.Host != nil {
		setClauses = append(setClauses, fmt.Sprintf("host = $%d", argIdx))
		args = append(args, *upd.Host)
		argIdx++
	}
	if upd.Port != nil {
		setClauses = append(setClauses, fmt.Sprintf("port = $%d", argIdx))
		args = append(args, *upd.Port)
		argIdx++
	}
	if upd.AuthType != nil {
		setClauses = append(setClauses, fmt.Sprintf("auth_type = $%d", argIdx))
		args = append(args, string(*upd.AuthType))
		argIdx++
	}
	if upd.UsernameEncrypted != nil {
		setClauses = append(setClauses, fmt.Sprintf("username_encrypted = $%d", argIdx))
		args = append(args, upd.UsernameEncrypted)
		argIdx++
		// Reset status when credentials change.
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(SMTPStatusUntested))
		argIdx++
	}
	if upd.PasswordEncrypted != nil {
		setClauses = append(setClauses, fmt.Sprintf("password_encrypted = $%d", argIdx))
		args = append(args, upd.PasswordEncrypted)
		argIdx++
		if upd.UsernameEncrypted == nil {
			// Only reset status if username didn't already reset it.
			setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
			args = append(args, string(SMTPStatusUntested))
			argIdx++
		}
	}
	if upd.TLSMode != nil {
		setClauses = append(setClauses, fmt.Sprintf("tls_mode = $%d", argIdx))
		args = append(args, string(*upd.TLSMode))
		argIdx++
	}
	if upd.TLSSkipVerify != nil {
		setClauses = append(setClauses, fmt.Sprintf("tls_skip_verify = $%d", argIdx))
		args = append(args, *upd.TLSSkipVerify)
		argIdx++
	}
	if upd.FromAddress != nil {
		setClauses = append(setClauses, fmt.Sprintf("from_address = $%d", argIdx))
		args = append(args, *upd.FromAddress)
		argIdx++
	}
	if upd.FromName != nil {
		setClauses = append(setClauses, fmt.Sprintf("from_name = $%d", argIdx))
		args = append(args, *upd.FromName)
		argIdx++
	}
	if upd.ReplyTo != nil {
		setClauses = append(setClauses, fmt.Sprintf("reply_to = $%d", argIdx))
		args = append(args, *upd.ReplyTo)
		argIdx++
	}
	if upd.CustomHELO != nil {
		setClauses = append(setClauses, fmt.Sprintf("custom_helo = $%d", argIdx))
		args = append(args, *upd.CustomHELO)
		argIdx++
	}
	if upd.MaxSendRate != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_send_rate = $%d", argIdx))
		args = append(args, *upd.MaxSendRate)
		argIdx++
	}
	if upd.MaxConnections != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_connections = $%d", argIdx))
		args = append(args, *upd.MaxConnections)
		argIdx++
	}
	if upd.TimeoutConnect != nil {
		setClauses = append(setClauses, fmt.Sprintf("timeout_connect = $%d", argIdx))
		args = append(args, *upd.TimeoutConnect)
		argIdx++
	}
	if upd.TimeoutSend != nil {
		setClauses = append(setClauses, fmt.Sprintf("timeout_send = $%d", argIdx))
		args = append(args, *upd.TimeoutSend)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE smtp_profiles SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, name, description, host, port, auth_type,
		          username_encrypted, password_encrypted,
		          tls_mode, tls_skip_verify,
		          from_address, from_name, reply_to, custom_helo,
		          max_send_rate, max_connections, timeout_connect, timeout_send,
		          status, status_message, last_tested_at, created_by, deleted_at, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	return r.scanOne(r.db.QueryRowContext(ctx, q, args...))
}

// Delete soft-deletes the SMTP profile by setting deleted_at.
func (r *SMTPProfileRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "UPDATE smtp_profiles SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("smtp profiles: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("smtp profiles: delete rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("smtp profiles: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateStatus updates the status, message, and last_tested_at timestamp.
func (r *SMTPProfileRepository) UpdateStatus(ctx context.Context, id string, status SMTPProfileStatus, message *string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE smtp_profiles SET status = $1, status_message = $2, last_tested_at = now() WHERE id = $3",
		string(status), message, id)
	if err != nil {
		return fmt.Errorf("smtp profiles: update status: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("smtp profiles: update status rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("smtp profiles: update status: %w", sql.ErrNoRows)
	}
	return nil
}

// GetActiveCampaignAssociations returns all campaign associations for the given profile.
func (r *SMTPProfileRepository) GetActiveCampaignAssociations(ctx context.Context, profileID string) ([]SMTPCampaignAssoc, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT campaign_id, smtp_profile_id FROM campaign_smtp_profiles WHERE smtp_profile_id = $1",
		profileID)
	if err != nil {
		return nil, fmt.Errorf("smtp profiles: get associations: %w", err)
	}
	defer rows.Close()

	var results []SMTPCampaignAssoc
	for rows.Next() {
		var a SMTPCampaignAssoc
		if err := rows.Scan(&a.CampaignID, &a.SMTPProfileID); err != nil {
			return nil, fmt.Errorf("smtp profiles: scan association: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("smtp profiles: associations rows: %w", err)
	}
	return results, nil
}

// CreateCampaignAssociation inserts a campaign-SMTP profile association.
func (r *SMTPProfileRepository) CreateCampaignAssociation(ctx context.Context, a CampaignSMTPProfile) (CampaignSMTPProfile, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO campaign_smtp_profiles
			(id, campaign_id, smtp_profile_id, priority, weight,
			 from_address_override, from_name_override, reply_to_override, segment_filter)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, campaign_id, smtp_profile_id, priority, weight,
		          from_address_override, from_name_override, reply_to_override, segment_filter, created_at`
	row := r.db.QueryRowContext(ctx, q,
		id, a.CampaignID, a.SMTPProfileID, a.Priority, a.Weight,
		a.FromAddressOverride, a.FromNameOverride, a.ReplyToOverride,
		nullBytes(a.SegmentFilter),
	)
	return r.scanAssocOne(row)
}

// GetCampaignAssociation returns a single campaign-SMTP association by its ID.
func (r *SMTPProfileRepository) GetCampaignAssociation(ctx context.Context, id string) (CampaignSMTPProfile, error) {
	const q = `
		SELECT id, campaign_id, smtp_profile_id, priority, weight,
		       from_address_override, from_name_override, reply_to_override, segment_filter, created_at
		FROM campaign_smtp_profiles WHERE id = $1`
	return r.scanAssocOne(r.db.QueryRowContext(ctx, q, id))
}

// ListCampaignAssociations returns all SMTP associations for a campaign.
func (r *SMTPProfileRepository) ListCampaignAssociations(ctx context.Context, campaignID string) ([]CampaignSMTPProfile, error) {
	const q = `
		SELECT id, campaign_id, smtp_profile_id, priority, weight,
		       from_address_override, from_name_override, reply_to_override, segment_filter, created_at
		FROM campaign_smtp_profiles WHERE campaign_id = $1 ORDER BY priority ASC`
	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("smtp profiles: list associations: %w", err)
	}
	defer rows.Close()

	var results []CampaignSMTPProfile
	for rows.Next() {
		a, err := r.scanAssocRow(rows)
		if err != nil {
			return nil, fmt.Errorf("smtp profiles: list associations scan: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("smtp profiles: list associations rows: %w", err)
	}
	return results, nil
}

// UpdateCampaignAssociation updates mutable fields on a campaign-SMTP association.
func (r *SMTPProfileRepository) UpdateCampaignAssociation(ctx context.Context, id string, priority, weight *int, fromAddr, fromName, replyTo *string, segmentFilter []byte) (CampaignSMTPProfile, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if priority != nil {
		setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, *priority)
		argIdx++
	}
	if weight != nil {
		setClauses = append(setClauses, fmt.Sprintf("weight = $%d", argIdx))
		args = append(args, *weight)
		argIdx++
	}
	if fromAddr != nil {
		setClauses = append(setClauses, fmt.Sprintf("from_address_override = $%d", argIdx))
		args = append(args, *fromAddr)
		argIdx++
	}
	if fromName != nil {
		setClauses = append(setClauses, fmt.Sprintf("from_name_override = $%d", argIdx))
		args = append(args, *fromName)
		argIdx++
	}
	if replyTo != nil {
		setClauses = append(setClauses, fmt.Sprintf("reply_to_override = $%d", argIdx))
		args = append(args, *replyTo)
		argIdx++
	}
	if segmentFilter != nil {
		setClauses = append(setClauses, fmt.Sprintf("segment_filter = $%d", argIdx))
		args = append(args, segmentFilter)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetCampaignAssociation(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE campaign_smtp_profiles SET %s WHERE id = $%d
		RETURNING id, campaign_id, smtp_profile_id, priority, weight,
		          from_address_override, from_name_override, reply_to_override, segment_filter, created_at`,
		strings.Join(setClauses, ", "), argIdx)

	return r.scanAssocOne(r.db.QueryRowContext(ctx, q, args...))
}

// DeleteCampaignAssociation removes a campaign-SMTP association by ID.
func (r *SMTPProfileRepository) DeleteCampaignAssociation(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM campaign_smtp_profiles WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("smtp profiles: delete association: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("smtp profiles: delete association rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("smtp profiles: delete association: %w", sql.ErrNoRows)
	}
	return nil
}

// UpsertSendSchedule inserts or replaces the send schedule for a campaign.
func (r *SMTPProfileRepository) UpsertSendSchedule(ctx context.Context, s CampaignSendSchedule) (CampaignSendSchedule, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO campaign_send_schedules
			(id, campaign_id, sending_strategy, send_window_start, send_window_end,
			 send_window_timezone, send_window_days, campaign_rate_limit,
			 min_delay_ms, max_delay_ms, batch_size, batch_pause_seconds)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (campaign_id) DO UPDATE SET
			sending_strategy     = EXCLUDED.sending_strategy,
			send_window_start    = EXCLUDED.send_window_start,
			send_window_end      = EXCLUDED.send_window_end,
			send_window_timezone = EXCLUDED.send_window_timezone,
			send_window_days     = EXCLUDED.send_window_days,
			campaign_rate_limit  = EXCLUDED.campaign_rate_limit,
			min_delay_ms         = EXCLUDED.min_delay_ms,
			max_delay_ms         = EXCLUDED.max_delay_ms,
			batch_size           = EXCLUDED.batch_size,
			batch_pause_seconds  = EXCLUDED.batch_pause_seconds
		RETURNING id, campaign_id, sending_strategy, send_window_start, send_window_end,
		          send_window_timezone, send_window_days, campaign_rate_limit,
		          min_delay_ms, max_delay_ms, batch_size, batch_pause_seconds,
		          created_at, updated_at`

	var days []int
	if s.SendWindowDays != nil {
		days = s.SendWindowDays
	}

	row := r.db.QueryRowContext(ctx, q,
		id, s.CampaignID, s.SendingStrategy,
		s.SendWindowStart, s.SendWindowEnd, s.SendWindowTimezone,
		intArrayToPostgres(days), s.CampaignRateLimit,
		s.MinDelayMs, s.MaxDelayMs, s.BatchSize, s.BatchPauseSeconds,
	)
	return r.scanScheduleOne(row)
}

// GetSendSchedule returns the send schedule for a campaign. Returns sql.ErrNoRows if not set.
func (r *SMTPProfileRepository) GetSendSchedule(ctx context.Context, campaignID string) (CampaignSendSchedule, error) {
	const q = `
		SELECT id, campaign_id, sending_strategy, send_window_start, send_window_end,
		       send_window_timezone, send_window_days, campaign_rate_limit,
		       min_delay_ms, max_delay_ms, batch_size, batch_pause_seconds,
		       created_at, updated_at
		FROM campaign_send_schedules WHERE campaign_id = $1`
	return r.scanScheduleOne(r.db.QueryRowContext(ctx, q, campaignID))
}

// --- scan helpers ---

func (r *SMTPProfileRepository) scanOne(row *sql.Row) (SMTPProfile, error) {
	var p SMTPProfile
	var usernameEnc, passwordEnc []byte
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Host, &p.Port, &p.AuthType,
		&usernameEnc, &passwordEnc,
		&p.TLSMode, &p.TLSSkipVerify,
		&p.FromAddress, &p.FromName, &p.ReplyTo, &p.CustomHELO,
		&p.MaxSendRate, &p.MaxConnections, &p.TimeoutConnect, &p.TimeoutSend,
		&p.Status, &p.StatusMessage, &p.LastTestedAt, &p.CreatedBy, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return SMTPProfile{}, fmt.Errorf("smtp profiles: scan: %w", err)
	}
	p.UsernameEncrypted = usernameEnc
	p.PasswordEncrypted = passwordEnc
	return p, nil
}

func (r *SMTPProfileRepository) scanRow(rows *sql.Rows) (SMTPProfile, error) {
	var p SMTPProfile
	var usernameEnc, passwordEnc []byte
	err := rows.Scan(
		&p.ID, &p.Name, &p.Description, &p.Host, &p.Port, &p.AuthType,
		&usernameEnc, &passwordEnc,
		&p.TLSMode, &p.TLSSkipVerify,
		&p.FromAddress, &p.FromName, &p.ReplyTo, &p.CustomHELO,
		&p.MaxSendRate, &p.MaxConnections, &p.TimeoutConnect, &p.TimeoutSend,
		&p.Status, &p.StatusMessage, &p.LastTestedAt, &p.CreatedBy, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return SMTPProfile{}, err
	}
	p.UsernameEncrypted = usernameEnc
	p.PasswordEncrypted = passwordEnc
	return p, nil
}

func (r *SMTPProfileRepository) scanAssocOne(row *sql.Row) (CampaignSMTPProfile, error) {
	var a CampaignSMTPProfile
	var segFilter []byte
	err := row.Scan(
		&a.ID, &a.CampaignID, &a.SMTPProfileID, &a.Priority, &a.Weight,
		&a.FromAddressOverride, &a.FromNameOverride, &a.ReplyToOverride,
		&segFilter, &a.CreatedAt,
	)
	if err != nil {
		return CampaignSMTPProfile{}, fmt.Errorf("smtp profiles: scan assoc: %w", err)
	}
	a.SegmentFilter = segFilter
	return a, nil
}

func (r *SMTPProfileRepository) scanAssocRow(rows *sql.Rows) (CampaignSMTPProfile, error) {
	var a CampaignSMTPProfile
	var segFilter []byte
	err := rows.Scan(
		&a.ID, &a.CampaignID, &a.SMTPProfileID, &a.Priority, &a.Weight,
		&a.FromAddressOverride, &a.FromNameOverride, &a.ReplyToOverride,
		&segFilter, &a.CreatedAt,
	)
	if err != nil {
		return CampaignSMTPProfile{}, err
	}
	a.SegmentFilter = segFilter
	return a, nil
}

func (r *SMTPProfileRepository) scanScheduleOne(row *sql.Row) (CampaignSendSchedule, error) {
	var s CampaignSendSchedule
	var daysStr *string
	err := row.Scan(
		&s.ID, &s.CampaignID, &s.SendingStrategy,
		&s.SendWindowStart, &s.SendWindowEnd, &s.SendWindowTimezone,
		&daysStr, &s.CampaignRateLimit,
		&s.MinDelayMs, &s.MaxDelayMs, &s.BatchSize, &s.BatchPauseSeconds,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return CampaignSendSchedule{}, fmt.Errorf("smtp schedules: scan: %w", err)
	}
	if daysStr != nil {
		s.SendWindowDays = parseIntArray(*daysStr)
	}
	return s, nil
}

// nullBytes converts a nil or empty byte slice to nil for SQL NULL handling.
func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// intArrayToPostgres converts a Go int slice to a PostgreSQL array literal string.
func intArrayToPostgres(vals []int) *string {
	if len(vals) == 0 {
		return nil
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	s := "{" + strings.Join(parts, ",") + "}"
	return &s
}

// parseIntArray parses a PostgreSQL array literal like "{0,1,5}" into a Go slice.
func parseIntArray(s string) []int {
	s = strings.Trim(s, "{}")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var v int
		fmt.Sscanf(p, "%d", &v)
		result = append(result, v)
	}
	return result
}
