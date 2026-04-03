package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultRetentionDays   = 90
	rateLimitPerUserPerHr  = 100
)

// rateLimitEntry tracks per-user notification counts for rate limiting.
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// RecipientSpec describes who should receive a notification.
type RecipientSpec struct {
	// UserIDs specifies explicit recipient user IDs.
	UserIDs []string
	// Role specifies a role name; all users with that role are recipients.
	Role string
}

// CreateNotificationParams holds all inputs for creating a notification.
type CreateNotificationParams struct {
	Category     string
	Severity     string
	Title        string
	Body         string
	ResourceType string // optional
	ResourceID   string // optional
	ActionURL    string // optional
	Recipients   RecipientSpec
}

// Notification is the in-memory representation returned by queries.
type Notification struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Category     string     `json:"category"`
	Severity     string     `json:"severity"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *string    `json:"resource_id,omitempty"`
	ActionURL    *string    `json:"action_url,omitempty"`
	IsRead       bool       `json:"is_read"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// wsMessage is the JSON envelope pushed to WebSocket clients.
type wsMessage struct {
	Type string       `json:"type"`
	Data Notification `json:"data"`
}

// NotificationService manages notification creation and WebSocket delivery.
type NotificationService struct {
	db             *sql.DB
	hub            *Hub
	emailSender    *EmailSender
	webhookSender  *WebhookSender

	rateMu    sync.Mutex
	rateLimit map[string]*rateLimitEntry // userID -> entry
}

// NewNotificationService creates a NotificationService.
func NewNotificationService(db *sql.DB, hub *Hub) *NotificationService {
	return &NotificationService{
		db:        db,
		hub:       hub,
		rateLimit: make(map[string]*rateLimitEntry),
	}
}

// SetEmailSender attaches the email delivery channel.
func (s *NotificationService) SetEmailSender(es *EmailSender) {
	s.emailSender = es
}

// SetWebhookSender attaches the webhook delivery channel.
func (s *NotificationService) SetWebhookSender(ws *WebhookSender) {
	s.webhookSender = ws
}

// Create resolves recipients, inserts per-user notification records asynchronously,
// and pushes to the WebSocket hub. Never blocks the caller.
func (s *NotificationService) Create(ctx context.Context, params CreateNotificationParams) {
	go func() {
		if err := s.create(context.Background(), params); err != nil {
			slog.Error("notification: create failed", "error", err)
		}
	}()
}

// create is the synchronous implementation called inside the goroutine.
func (s *NotificationService) create(ctx context.Context, params CreateNotificationParams) error {
	userIDs, err := s.resolveRecipients(ctx, params.Recipients)
	if err != nil {
		return fmt.Errorf("notification: resolve recipients: %w", err)
	}
	if len(userIDs) == 0 {
		return nil
	}

	expiresAt := time.Now().UTC().Add(defaultRetentionDays * 24 * time.Hour)

	for _, uid := range userIDs {
		if s.isRateLimited(uid) {
			slog.Debug("notification: rate limited", "user_id", uid)
			continue
		}
		n, err := s.insertNotification(ctx, uid, params, expiresAt)
		if err != nil {
			slog.Error("notification: insert failed", "user_id", uid, "error", err)
			continue
		}
		s.pushToHub(uid, n)
		s.deliverEmail(ctx, uid, n)
	}

	// Webhook delivery — fires once per notification (not per user).
	if s.webhookSender != nil && len(userIDs) > 0 {
		// Use the first user's notification as representative for the webhook payload.
		representative := &Notification{
			ID:        uuid.New().String(),
			Category:  params.Category,
			Severity:  params.Severity,
			Title:     params.Title,
			Body:      params.Body,
			CreatedAt: time.Now().UTC(),
		}
		if params.Severity == "" {
			representative.Severity = "info"
		}
		s.webhookSender.DeliverToMatchingEndpoints(ctx, representative)
	}

	return nil
}

// resolveRecipients returns the deduplicated set of user IDs for the given spec.
func (s *NotificationService) resolveRecipients(ctx context.Context, spec RecipientSpec) ([]string, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0, len(spec.UserIDs))

	for _, uid := range spec.UserIDs {
		if _, dup := seen[uid]; !dup {
			seen[uid] = struct{}{}
			ids = append(ids, uid)
		}
	}

	if spec.Role != "" {
		rows, err := s.db.QueryContext(ctx, `
			SELECT DISTINCT ur.user_id
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE r.name = $1`, spec.Role)
		if err != nil {
			return nil, fmt.Errorf("resolve role recipients: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				return nil, err
			}
			if _, dup := seen[uid]; !dup {
				seen[uid] = struct{}{}
				ids = append(ids, uid)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return ids, nil
}

// insertNotification persists a single notification row and returns the populated struct.
func (s *NotificationService) insertNotification(ctx context.Context, userID string, p CreateNotificationParams, expiresAt time.Time) (*Notification, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	severity := p.Severity
	if severity == "" {
		severity = "info"
	}

	var resourceType, resourceID, actionURL sql.NullString
	if p.ResourceType != "" {
		resourceType = sql.NullString{String: p.ResourceType, Valid: true}
	}
	if p.ResourceID != "" {
		resourceID = sql.NullString{String: p.ResourceID, Valid: true}
	}
	if p.ActionURL != "" {
		actionURL = sql.NullString{String: p.ActionURL, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notifications
			(id, user_id, category, severity, title, body, resource_type, resource_id, action_url, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id, userID, p.Category, severity, p.Title, p.Body,
		resourceType, resourceID, actionURL, expiresAt, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}

	n := &Notification{
		ID:        id,
		UserID:    userID,
		Category:  p.Category,
		Severity:  severity,
		Title:     p.Title,
		Body:      p.Body,
		IsRead:    false,
		ExpiresAt: &expiresAt,
		CreatedAt: now,
	}
	if resourceType.Valid {
		s := resourceType.String
		n.ResourceType = &s
	}
	if resourceID.Valid {
		s := resourceID.String
		n.ResourceID = &s
	}
	if actionURL.Valid {
		s := actionURL.String
		n.ActionURL = &s
	}
	return n, nil
}

// isRateLimited returns true if the user has exceeded the per-hour notification limit.
// When the limit is first reached, a summary notification is inserted.
func (s *NotificationService) isRateLimited(userID string) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	now := time.Now()
	entry, ok := s.rateLimit[userID]
	if !ok || now.After(entry.resetTime) {
		s.rateLimit[userID] = &rateLimitEntry{count: 1, resetTime: now.Add(time.Hour)}
		return false
	}
	entry.count++
	if entry.count == rateLimitPerUserPerHr+1 && s.db != nil {
		// Insert a summary notification on the first suppression.
		go func() {
			_, err := s.insertNotification(context.Background(), userID, CreateNotificationParams{
				Category: "system",
				Severity: "warning",
				Title:    "Notifications suppressed",
				Body:     fmt.Sprintf("More than %d notifications in the last hour — additional notifications are being suppressed.", rateLimitPerUserPerHr),
			}, now.Add(defaultRetentionDays*24*time.Hour))
			if err != nil {
				slog.Error("notification: insert rate limit summary", "user_id", userID, "error", err)
			}
		}()
	}
	return entry.count > rateLimitPerUserPerHr
}

// deliverEmail checks user preference and sends a notification email if opted in.
func (s *NotificationService) deliverEmail(ctx context.Context, userID string, n *Notification) {
	if s.emailSender == nil {
		return
	}
	enabled, mode, err := s.emailSender.CheckUserPreference(ctx, userID, n.Category)
	if err != nil {
		slog.Error("notification: check email preference", "user_id", userID, "error", err)
		return
	}
	if !enabled {
		return
	}
	// For digest mode, skip immediate send — a separate worker handles batching.
	if mode == "digest" {
		return
	}
	email, err := s.emailSender.GetUserEmail(ctx, userID)
	if err != nil || email == "" {
		return
	}
	actionURL := ""
	if n.ActionURL != nil {
		actionURL = *n.ActionURL
	}
	if err := s.emailSender.SendNotificationEmail(ctx, email, n.Title, n.Body, actionURL); err != nil {
		slog.Error("notification: send email", "user_id", userID, "error", err)
	}
}

// pushToHub marshals the notification and sends it via the WebSocket hub.
func (s *NotificationService) pushToHub(userID string, n *Notification) {
	msg := wsMessage{Type: "notification", Data: *n}
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("notification: marshal ws message", "id", n.ID, "error", err)
		return
	}
	s.hub.SendToUser(userID, payload)
}
