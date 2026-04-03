package authprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// FusionAuthProvider handles OAuth2/OIDC login via FusionAuth.
// It reuses OIDCProvider internally for the Authorization Code Flow,
// adding FusionAuth-specific endpoints and group extraction.
type FusionAuthProvider struct {
	oidc *OIDCProvider
}

// NewFusionAuthProvider creates a FusionAuthProvider.
func NewFusionAuthProvider() *FusionAuthProvider {
	return &FusionAuthProvider{oidc: NewOIDCProvider()}
}

// InitiateAuthFlow builds the FusionAuth authorization redirect URL.
func (p *FusionAuthProvider) InitiateAuthFlow(ctx context.Context, cfg FusionAuthConfig, linking bool, userID string) (string, string, error) {
	oidcCfg := p.toOIDCConfig(cfg)
	return p.oidc.InitiateAuthFlow(ctx, oidcCfg, linking, userID)
}

// HandleCallback processes the FusionAuth callback, extracts claims,
// and fetches group memberships via the FusionAuth API.
func (p *FusionAuthProvider) HandleCallback(ctx context.Context, cfg FusionAuthConfig, code, state string) (ExternalClaims, bool, string, error) {
	oidcCfg := p.toOIDCConfig(cfg)
	claims, linking, linkUserID, err := p.oidc.HandleCallback(ctx, oidcCfg, code, state)
	if err != nil {
		return ExternalClaims{}, false, "", err
	}

	// Fetch group memberships via the FusionAuth admin API if an API key is set.
	if cfg.APIKey != "" && claims.Subject != "" {
		groups, ferr := p.fetchGroups(ctx, cfg, claims.Subject)
		if ferr == nil {
			claims.Groups = groups
		}
		// Non-fatal: log but continue if group fetch fails.
	}

	return claims, linking, linkUserID, nil
}

// TestConnection verifies the FusionAuth application exists and is reachable.
func (p *FusionAuthProvider) TestConnection(ctx context.Context, cfg FusionAuthConfig) TestResult {
	if cfg.APIKey == "" || cfg.ApplicationID == "" {
		// Fall back to OIDC discovery test.
		return p.oidc.TestConnection(ctx, p.toOIDCConfig(cfg))
	}

	url := fmt.Sprintf("%s/api/application/%s", cfg.BaseURL, cfg.ApplicationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return TestResult{StageReached: "request_build", ErrorDetail: err.Error()}
	}
	req.Header.Set("Authorization", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{StageReached: "connect", ErrorDetail: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TestResult{StageReached: "api_check", ErrorDetail: fmt.Sprintf("status %d", resp.StatusCode)}
	}
	return TestResult{Success: true, StageReached: "api_check"}
}

// fetchGroups retrieves the group memberships for a user via the FusionAuth API.
func (p *FusionAuthProvider) fetchGroups(ctx context.Context, cfg FusionAuthConfig, userID string) ([]string, error) {
	url := fmt.Sprintf("%s/api/user/%s", cfg.BaseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fusionauth fetch groups: build request: %w", err)
	}
	req.Header.Set("Authorization", cfg.APIKey)
	if cfg.TenantID != "" {
		req.Header.Set("X-FusionAuth-TenantId", cfg.TenantID)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fusionauth fetch groups: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fusionauth fetch groups: status %d", resp.StatusCode)
	}

	var payload struct {
		User struct {
			Memberships []struct {
				GroupID string `json:"groupId"`
				Group   struct {
					Name string `json:"name"`
				} `json:"group"`
			} `json:"memberships"`
			Registrations []struct {
				Roles []string `json:"roles"`
			} `json:"registrations"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("fusionauth fetch groups: unmarshal: %w", err)
	}

	var groups []string
	for _, m := range payload.User.Memberships {
		if m.Group.Name != "" {
			groups = append(groups, m.Group.Name)
		} else if m.GroupID != "" {
			groups = append(groups, m.GroupID)
		}
	}
	// Also include application roles.
	for _, reg := range payload.User.Registrations {
		groups = append(groups, reg.Roles...)
	}
	return groups, nil
}

// toOIDCConfig converts a FusionAuthConfig to an OIDCConfig using FusionAuth endpoints.
func (p *FusionAuthProvider) toOIDCConfig(cfg FusionAuthConfig) OIDCConfig {
	return OIDCConfig{
		IssuerURL:             cfg.BaseURL,
		ClientID:              cfg.ClientID,
		ClientSecret:          cfg.ClientSecret,
		Scopes:                []string{oidc.ScopeOpenID, "profile", "email"},
		RedirectURI:           cfg.RedirectURI,
		AuthorizationEndpoint: cfg.BaseURL + "/oauth2/authorize",
		TokenEndpoint:         cfg.BaseURL + "/oauth2/token",
		UserinfoEndpoint:      cfg.BaseURL + "/oauth2/userinfo",
	}
}

