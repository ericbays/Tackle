package authprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// AuthProviderConfigDTO is the API-safe representation of a provider configuration.
// Sensitive fields (client_secret, api_key, bind_password) are masked.
type AuthProviderConfigDTO struct {
	ID            string                   `json:"id"`
	Type          repositories.AuthProviderType `json:"type"`
	Name          string                   `json:"name"`
	Enabled       bool                     `json:"enabled"`
	DefaultRoleID *string                  `json:"default_role_id,omitempty"`
	AutoProvision bool                     `json:"auto_provision"`
	AuthOrder     repositories.AuthOrder   `json:"auth_order"`
	Config        map[string]any           `json:"config"`
}

// AuthProviderSummary is the minimal representation shown on the login page.
type AuthProviderSummary struct {
	ID          string                        `json:"id"`
	Type        repositories.AuthProviderType `json:"type"`
	Name        string                        `json:"name"`
	ButtonLabel string                        `json:"button_label"`
}

// ProviderInput is the deserialized request body for create/update operations.
type ProviderInput struct {
	Type          repositories.AuthProviderType `json:"type"`
	Name          string                        `json:"name"`
	Enabled       bool                          `json:"enabled"`
	DefaultRoleID *string                       `json:"default_role_id,omitempty"`
	AutoProvision bool                          `json:"auto_provision"`
	AuthOrder     repositories.AuthOrder        `json:"auth_order"`
	// Raw config map — validated and stored as encrypted JSON.
	Config map[string]any `json:"config"`
}

// Service manages auth provider configurations.
type Service struct {
	repo    *repositories.AuthProviderRepository
	rmRepo  *repositories.RoleMappingRepository
	idRepo  *repositories.AuthIdentityRepository
	enc     *credentials.AuthProviderEncryptionService
	audit   *auditsvc.AuditService
	oidc    *OIDCProvider
	fa      *FusionAuthProvider
	ldap    *LDAPProvider
	provSvc *ProvisioningService
	linkSvc *LinkingService
}

// NewService creates a new auth provider Service.
func NewService(
	repo *repositories.AuthProviderRepository,
	rmRepo *repositories.RoleMappingRepository,
	idRepo *repositories.AuthIdentityRepository,
	enc *credentials.AuthProviderEncryptionService,
	audit *auditsvc.AuditService,
) *Service {
	return &Service{
		repo:   repo,
		rmRepo: rmRepo,
		idRepo: idRepo,
		enc:    enc,
		audit:  audit,
		oidc:   NewOIDCProvider(),
		fa:     NewFusionAuthProvider(),
		ldap:   NewLDAPProvider(),
	}
}

// CreateProviderConfig creates and persists a new provider configuration.
func (s *Service) CreateProviderConfig(ctx context.Context, actorID string, input ProviderInput) (AuthProviderConfigDTO, error) {
	if err := validateInput(input); err != nil {
		return AuthProviderConfigDTO{}, err
	}

	encrypted, err := s.encryptConfig(input.Config)
	if err != nil {
		return AuthProviderConfigDTO{}, fmt.Errorf("create auth provider: %w", err)
	}

	authOrder := input.AuthOrder
	if authOrder == "" {
		authOrder = repositories.AuthOrderLocalFirst
	}

	p, err := s.repo.Create(ctx, repositories.AuthProvider{
		Type:          input.Type,
		Name:          input.Name,
		Configuration: encrypted,
		Enabled:       input.Enabled,
		DefaultRoleID: input.DefaultRoleID,
		AutoProvision: input.AutoProvision,
		AuthOrder:     authOrder,
	})
	if err != nil {
		return AuthProviderConfigDTO{}, fmt.Errorf("create auth provider: %w", err)
	}

	s.emitAudit(ctx, actorID, "auth.provider.created", map[string]any{
		"provider_id": p.ID, "type": p.Type, "name": p.Name,
	})

	return s.toDTO(p, input.Config), nil
}

// UpdateProviderConfig updates a provider configuration.
func (s *Service) UpdateProviderConfig(ctx context.Context, actorID, id string, input ProviderInput) (AuthProviderConfigDTO, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return AuthProviderConfigDTO{}, fmt.Errorf("update auth provider: %w", err)
	}

	encrypted, err := s.encryptConfig(input.Config)
	if err != nil {
		return AuthProviderConfigDTO{}, fmt.Errorf("update auth provider: %w", err)
	}

	authOrder := input.AuthOrder
	if authOrder == "" {
		authOrder = existing.AuthOrder
	}

	updated, err := s.repo.Update(ctx, id, repositories.AuthProvider{
		Name:          input.Name,
		Configuration: encrypted,
		Enabled:       input.Enabled,
		DefaultRoleID: input.DefaultRoleID,
		AutoProvision: input.AutoProvision,
		AuthOrder:     authOrder,
	})
	if err != nil {
		return AuthProviderConfigDTO{}, fmt.Errorf("update auth provider: %w", err)
	}

	s.emitAudit(ctx, actorID, "auth.provider.updated", map[string]any{
		"provider_id": id, "name": input.Name,
	})

	return s.toDTO(updated, input.Config), nil
}

// DeleteProviderConfig removes a provider configuration after checking for linked identities.
func (s *Service) DeleteProviderConfig(ctx context.Context, actorID, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete auth provider: %w", err)
	}

	s.emitAudit(ctx, actorID, "auth.provider.deleted", map[string]any{
		"provider_id": id, "type": p.Type, "name": p.Name,
	})

	return s.repo.Delete(ctx, id)
}

// EnableProviderConfig enables a provider.
func (s *Service) EnableProviderConfig(ctx context.Context, actorID, id string) error {
	if err := s.repo.Enable(ctx, id); err != nil {
		return fmt.Errorf("enable auth provider: %w", err)
	}
	s.emitAudit(ctx, actorID, "auth.provider.enabled", map[string]any{"provider_id": id})
	return nil
}

// DisableProviderConfig disables a provider.
func (s *Service) DisableProviderConfig(ctx context.Context, actorID, id string) error {
	if err := s.repo.Disable(ctx, id); err != nil {
		return fmt.Errorf("disable auth provider: %w", err)
	}
	s.emitAudit(ctx, actorID, "auth.provider.disabled", map[string]any{"provider_id": id})
	return nil
}

// GetProviderConfig returns a single provider with secrets masked.
func (s *Service) GetProviderConfig(ctx context.Context, id string) (AuthProviderConfigDTO, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return AuthProviderConfigDTO{}, err
	}
	cfg, err := s.decryptConfig(p.Configuration)
	if err != nil {
		return AuthProviderConfigDTO{}, err
	}
	return s.toDTO(p, maskSecrets(cfg)), nil
}

// ListProviderConfigs returns all providers with secrets masked.
func (s *Service) ListProviderConfigs(ctx context.Context) ([]AuthProviderConfigDTO, error) {
	providers, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AuthProviderConfigDTO, 0, len(providers))
	for _, p := range providers {
		cfg, err := s.decryptConfig(p.Configuration)
		if err != nil {
			slog.Warn("auth provider: decrypt config failed", "provider_id", p.ID, "error", err)
			cfg = map[string]any{}
		}
		out = append(out, s.toDTO(p, maskSecrets(cfg)))
	}
	return out, nil
}

// GetEnabledProviders returns summary info for all enabled providers (login page).
func (s *Service) GetEnabledProviders(ctx context.Context) ([]AuthProviderSummary, error) {
	providers, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AuthProviderSummary, 0, len(providers))
	for _, p := range providers {
		out = append(out, AuthProviderSummary{
			ID:          p.ID,
			Type:        p.Type,
			Name:        p.Name,
			ButtonLabel: buttonLabel(p.Type, p.Name),
		})
	}
	return out, nil
}

// TestProviderConfig tests connectivity for a given provider.
func (s *Service) TestProviderConfig(ctx context.Context, actorID, id string) (TestResult, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return TestResult{}, err
	}

	cfgMap, err := s.decryptConfig(p.Configuration)
	if err != nil {
		return TestResult{}, err
	}

	var result TestResult
	switch p.Type {
	case repositories.AuthProviderOIDC:
		var cfg OIDCConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return TestResult{}, fmt.Errorf("test provider: unmarshal oidc config: %w", err)
		}
		result = s.oidc.TestConnection(ctx, cfg)
	case repositories.AuthProviderFusionAuth:
		var cfg FusionAuthConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return TestResult{}, fmt.Errorf("test provider: unmarshal fusionauth config: %w", err)
		}
		result = s.fa.TestConnection(ctx, cfg)
	case repositories.AuthProviderLDAP:
		var cfg LDAPConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return TestResult{}, fmt.Errorf("test provider: unmarshal ldap config: %w", err)
		}
		result = s.ldap.TestConnection(ctx, cfg)
	default:
		return TestResult{}, fmt.Errorf("unknown provider type: %s", p.Type)
	}

	action := "auth_provider.test_failure"
	if result.Success {
		action = "auth_provider.test_success"
	}
	s.emitAudit(ctx, actorID, action, map[string]any{
		"provider_id": id, "stage": result.StageReached, "error": result.ErrorDetail,
	})

	return result, nil
}

// SetRoleMappings atomically replaces the role mappings for a provider.
func (s *Service) SetRoleMappings(ctx context.Context, actorID, providerID string, mappings []repositories.RoleMapping) error {
	if err := s.rmRepo.SetMappings(ctx, providerID, mappings); err != nil {
		return fmt.Errorf("set role mappings: %w", err)
	}
	s.emitAudit(ctx, actorID, "auth.provider.role_mappings_updated", map[string]any{
		"provider_id": providerID, "count": len(mappings),
	})
	return nil
}

// GetRoleMappings returns all role mappings for a provider.
func (s *Service) GetRoleMappings(ctx context.Context, providerID string) ([]repositories.RoleMapping, error) {
	return s.rmRepo.GetMappingsForProvider(ctx, providerID)
}

// InitiateOIDCFlow starts the OIDC/FusionAuth Authorization Code Flow for a given provider.
// linking indicates whether this is an account-link flow; userID is the current user (linking only).
func (s *Service) InitiateOIDCFlow(ctx context.Context, providerID string, linking bool, userID string) (string, string, error) {
	p, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		return "", "", fmt.Errorf("initiate oidc flow: %w", err)
	}
	cfgMap, err := s.decryptConfig(p.Configuration)
	if err != nil {
		return "", "", fmt.Errorf("initiate oidc flow: %w", err)
	}

	switch p.Type {
	case repositories.AuthProviderOIDC:
		var cfg OIDCConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return "", "", fmt.Errorf("initiate oidc flow: %w", err)
		}
		return s.oidc.InitiateAuthFlow(ctx, cfg, linking, userID)
	case repositories.AuthProviderFusionAuth:
		var cfg FusionAuthConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return "", "", fmt.Errorf("initiate oidc flow: %w", err)
		}
		return s.fa.InitiateAuthFlow(ctx, cfg, linking, userID)
	default:
		return "", "", fmt.Errorf("provider %s does not support OIDC flow", p.Type)
	}
}

// HandleOIDCCallback completes the OIDC/FusionAuth callback and returns a provisioned user.
// The ProvisioningService must be set via SetProvisioningService before calling this.
func (s *Service) HandleOIDCCallback(ctx context.Context, providerID, code, state string) (ProvisionedUser, error) {
	if s.provSvc == nil {
		return ProvisionedUser{}, fmt.Errorf("handle oidc callback: provisioning service not set")
	}
	p, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("handle oidc callback: %w", err)
	}
	cfgMap, err := s.decryptConfig(p.Configuration)
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("handle oidc callback: decrypt: %w", err)
	}

	var (
		claims    ExternalClaims
		linking   bool
		linkUID   string
	)

	switch p.Type {
	case repositories.AuthProviderOIDC:
		var cfg OIDCConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return ProvisionedUser{}, fmt.Errorf("handle oidc callback: %w", err)
		}
		claims, linking, linkUID, err = s.oidc.HandleCallback(ctx, cfg, code, state)
	case repositories.AuthProviderFusionAuth:
		var cfg FusionAuthConfig
		if err := remarshal(cfgMap, &cfg); err != nil {
			return ProvisionedUser{}, fmt.Errorf("handle oidc callback: %w", err)
		}
		claims, linking, linkUID, err = s.fa.HandleCallback(ctx, cfg, code, state)
	default:
		return ProvisionedUser{}, fmt.Errorf("provider %s does not support OIDC callback", p.Type)
	}
	if err != nil {
		s.emitAudit(ctx, "", "auth.oidc_login_failure", map[string]any{
			"provider_id": providerID, "error": err.Error(),
		})
		return ProvisionedUser{}, fmt.Errorf("handle oidc callback: %w", err)
	}

	if linking {
		// Account-link flow — complete the link and return minimal result.
		if err := s.linkSvc.CompleteAccountLink(ctx, linkUID, p, claims); err != nil {
			return ProvisionedUser{}, fmt.Errorf("handle oidc callback: link: %w", err)
		}
		return ProvisionedUser{UserID: linkUID}, nil
	}

	provisioned, err := s.provSvc.ResolveExternalUser(ctx, p, claims)
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("handle oidc callback: provision: %w", err)
	}
	if !provisioned.NeedsLinking {
		s.emitAudit(ctx, provisioned.UserID, "auth.oidc_login_success", map[string]any{
			"provider_id": providerID, "user_id": provisioned.UserID,
		})
	}
	return provisioned, nil
}

// HandleLinkCallback completes an account-link callback flow.
func (s *Service) HandleLinkCallback(ctx context.Context, providerID, code, state string) error {
	_, err := s.HandleOIDCCallback(ctx, providerID, code, state)
	return err
}

// SetProvisioningService wires the ProvisioningService dependency (to avoid circular init).
func (s *Service) SetProvisioningService(provSvc *ProvisioningService) {
	s.provSvc = provSvc
}

// SetLinkingService wires the LinkingService dependency.
func (s *Service) SetLinkingService(linkSvc *LinkingService) {
	s.linkSvc = linkSvc
}

// --- helpers ---

func (s *Service) encryptConfig(cfg map[string]any) ([]byte, error) {
	return s.enc.Encrypt(cfg)
}

func (s *Service) decryptConfig(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := s.enc.Decrypt(data, &m); err != nil {
		return nil, fmt.Errorf("decrypt provider config: %w", err)
	}
	return m, nil
}

func (s *Service) toDTO(p repositories.AuthProvider, cfgMap map[string]any) AuthProviderConfigDTO {
	return AuthProviderConfigDTO{
		ID:            p.ID,
		Type:          p.Type,
		Name:          p.Name,
		Enabled:       p.Enabled,
		DefaultRoleID: p.DefaultRoleID,
		AutoProvision: p.AutoProvision,
		AuthOrder:     p.AuthOrder,
		Config:        cfgMap,
	}
}

func (s *Service) emitAudit(ctx context.Context, actorID, action string, details map[string]any) {
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

// secretFields lists config keys that should be masked in API responses.
var secretFields = []string{"client_secret", "api_key", "bind_password"}

func maskSecrets(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		masked := false
		for _, sf := range secretFields {
			if strings.EqualFold(k, sf) {
				if s, ok := v.(string); ok && s != "" {
					out[k] = "***"
				} else {
					out[k] = v
				}
				masked = true
				break
			}
		}
		if !masked {
			out[k] = v
		}
	}
	return out
}

func buttonLabel(t repositories.AuthProviderType, name string) string {
	switch t {
	case repositories.AuthProviderOIDC:
		return "Sign in with " + name
	case repositories.AuthProviderFusionAuth:
		return "Sign in with " + name
	case repositories.AuthProviderLDAP:
		return "Sign in with " + name + " (LDAP)"
	default:
		return "Sign in with " + name
	}
}

func validateInput(input ProviderInput) error {
	if input.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	switch input.Type {
	case repositories.AuthProviderOIDC, repositories.AuthProviderFusionAuth, repositories.AuthProviderLDAP:
	default:
		return fmt.Errorf("unsupported provider type: %s", input.Type)
	}
	return nil
}

// remarshal round-trips a map through JSON into a typed struct.
func remarshal(src any, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
