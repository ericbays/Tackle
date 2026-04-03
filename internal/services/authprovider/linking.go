package authprovider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// LinkedIdentityDTO is the API-safe representation of a linked auth identity.
type LinkedIdentityDTO struct {
	ID               string                        `json:"id"`
	ProviderType     repositories.AuthProviderType `json:"provider_type"`
	ProviderConfigID string                        `json:"provider_config_id"`
	ProviderName     string                        `json:"provider_name,omitempty"`
	ExternalEmail    *string                       `json:"external_email,omitempty"`
}

// LinkingService manages account linking and unlinking.
type LinkingService struct {
	db     *sql.DB
	idRepo *repositories.AuthIdentityRepository
	audit  *auditsvc.AuditService
}

// NewLinkingService creates a LinkingService.
func NewLinkingService(
	db *sql.DB,
	idRepo *repositories.AuthIdentityRepository,
	audit *auditsvc.AuditService,
) *LinkingService {
	return &LinkingService{db: db, idRepo: idRepo, audit: audit}
}

// CompleteAccountLink links an external identity to a local user account.
// It rejects the link if the external identity is already linked to a different user.
func (s *LinkingService) CompleteAccountLink(
	ctx context.Context,
	userID string,
	provider repositories.AuthProvider,
	claims ExternalClaims,
) error {
	// Verify there is no existing link for this external identity.
	existing, err := s.idRepo.GetByExternalSubject(ctx, provider.ID, claims.Subject)
	if err == nil && existing.UserID != userID {
		return fmt.Errorf("complete account link: external identity already linked to another user")
	}
	if err == nil && existing.UserID == userID {
		// Already linked to this user — idempotent, no-op.
		return nil
	}
	if !strings.Contains(err.Error(), "auth identity not found") {
		return fmt.Errorf("complete account link: lookup: %w", err)
	}

	email := &claims.Email
	if claims.Email == "" {
		email = nil
	}

	if _, err := s.idRepo.Create(ctx, repositories.AuthIdentity{
		UserID:           userID,
		ProviderType:     provider.Type,
		ProviderConfigID: provider.ID,
		ExternalSubject:  claims.Subject,
		ExternalEmail:    email,
	}); err != nil {
		return fmt.Errorf("complete account link: create identity: %w", err)
	}

	s.emitAudit(ctx, userID, "auth.account.linked", map[string]any{
		"user_id": userID, "provider_id": provider.ID, "provider_type": string(provider.Type),
	})
	return nil
}

// UnlinkIdentity removes a linked identity, but only if at least one auth method remains.
func (s *LinkingService) UnlinkIdentity(ctx context.Context, userID, identityID string) error {
	// Count remaining identities after removal.
	count, err := s.idRepo.CountByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("unlink identity: count: %w", err)
	}

	// Check if user has a local password.
	hasPassword, err := s.userHasPassword(ctx, userID)
	if err != nil {
		return fmt.Errorf("unlink identity: check password: %w", err)
	}

	// count includes the identity we're about to remove; after removal = count - 1.
	remaining := count - 1
	if remaining == 0 && !hasPassword {
		s.emitAudit(ctx, userID, "auth.account.unlink_rejected", map[string]any{
			"user_id": userID, "identity_id": identityID, "reason": "last_method",
		})
		return fmt.Errorf("cannot unlink last authentication method")
	}

	if err := s.idRepo.Delete(ctx, identityID); err != nil {
		return fmt.Errorf("unlink identity: %w", err)
	}

	s.emitAudit(ctx, userID, "auth.account.unlinked", map[string]any{
		"user_id": userID, "identity_id": identityID,
	})
	return nil
}

// GetLinkedIdentities returns all linked identities for a user.
func (s *LinkingService) GetLinkedIdentities(ctx context.Context, userID string) ([]LinkedIdentityDTO, error) {
	identities, err := s.idRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]LinkedIdentityDTO, 0, len(identities))
	for _, i := range identities {
		out = append(out, LinkedIdentityDTO{
			ID:               i.ID,
			ProviderType:     i.ProviderType,
			ProviderConfigID: i.ProviderConfigID,
			ExternalEmail:    i.ExternalEmail,
		})
	}
	return out, nil
}

func (s *LinkingService) userHasPassword(ctx context.Context, userID string) (bool, error) {
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if err != nil {
		return false, fmt.Errorf("check user password: %w", err)
	}
	return hash.Valid && hash.String != "", nil
}

func (s *LinkingService) emitAudit(ctx context.Context, actorID, action string, details map[string]any) {
	if s.audit == nil {
		return
	}
	entry := auditsvc.LogEntry{
		Category:  auditsvc.CategoryUserActivity,
		Severity:  auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser,
		Action:    action,
		Details:   details,
	}
	if actorID != "" {
		entry.ActorID = &actorID
	}
	_ = s.audit.Log(ctx, entry)
}
