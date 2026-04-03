package authprovider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// pendingState stores the CSRF state and nonce for an in-flight OIDC auth request.
type pendingState struct {
	nonce    string
	linking  bool   // true if this is an account-link flow rather than login
	userID   string // non-empty for linking flows
	expiry   time.Time
}

// OIDCProvider handles the Authorization Code Flow for a generic OIDC provider.
type OIDCProvider struct {
	mu      sync.Mutex
	pending map[string]pendingState // keyed by state parameter
}

// NewOIDCProvider creates an OIDCProvider.
func NewOIDCProvider() *OIDCProvider {
	p := &OIDCProvider{pending: make(map[string]pendingState)}
	go p.gcLoop()
	return p
}

// InitiateAuthFlow builds the authorization redirect URL for the given OIDC config.
// Returns the redirect URL and the state token to store on the client.
func (p *OIDCProvider) InitiateAuthFlow(ctx context.Context, cfg OIDCConfig, linking bool, userID string) (redirectURL string, state string, err error) {
	provider, oauth2Cfg, err := p.buildProvider(ctx, cfg)
	if err != nil {
		return "", "", fmt.Errorf("oidc initiate: %w", err)
	}

	state, err = randomToken()
	if err != nil {
		return "", "", fmt.Errorf("oidc initiate: state: %w", err)
	}
	nonce, err := randomToken()
	if err != nil {
		return "", "", fmt.Errorf("oidc initiate: nonce: %w", err)
	}

	_ = provider // used implicitly via oauth2Cfg endpoint
	p.mu.Lock()
	p.pending[state] = pendingState{
		nonce:   nonce,
		linking: linking,
		userID:  userID,
		expiry:  time.Now().Add(5 * time.Minute),
	}
	p.mu.Unlock()

	url := oauth2Cfg.AuthCodeURL(state, oidc.Nonce(nonce))
	return url, state, nil
}

// HandleCallback validates the callback code+state, exchanges for tokens,
// and returns the normalized claims.
func (p *OIDCProvider) HandleCallback(ctx context.Context, cfg OIDCConfig, code, state string) (ExternalClaims, bool, string, error) {
	p.mu.Lock()
	ps, ok := p.pending[state]
	if ok {
		delete(p.pending, state)
	}
	p.mu.Unlock()

	if !ok || time.Now().After(ps.expiry) {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: invalid or expired state")
	}

	provider, oauth2Cfg, err := p.buildProvider(ctx, cfg)
	if err != nil {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: build provider: %w", err)
	}

	token, err := oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: exchange: %w", err)
	}

	rawIDToken, ok2 := token.Extra("id_token").(string)
	if !ok2 {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: no id_token in response")
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: verify id token: %w", err)
	}

	if idToken.Nonce != ps.nonce {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: nonce mismatch")
	}

	var claims struct {
		Subject           string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return ExternalClaims{}, false, "", fmt.Errorf("oidc callback: extract claims: %w", err)
	}

	ext := ExternalClaims{
		Subject:  idToken.Subject,
		Email:    claims.Email,
		Name:     claims.Name,
		Username: claims.PreferredUsername,
	}
	return ext, ps.linking, ps.userID, nil
}

// TestConnection attempts OIDC discovery to validate the issuer URL.
func (p *OIDCProvider) TestConnection(ctx context.Context, cfg OIDCConfig) TestResult {
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, _, err := p.buildProvider(ctx2, cfg)
	if err != nil {
		return TestResult{StageReached: "discovery", ErrorDetail: err.Error()}
	}
	return TestResult{Success: true, StageReached: "discovery"}
}

func (p *OIDCProvider) buildProvider(ctx context.Context, cfg OIDCConfig) (*oidc.Provider, *oauth2.Config, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	ctx = oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc discovery for %s: %w", cfg.IssuerURL, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	// Allow manual endpoint overrides.
	if cfg.AuthorizationEndpoint != "" {
		oauth2Cfg.Endpoint.AuthURL = cfg.AuthorizationEndpoint
	}
	if cfg.TokenEndpoint != "" {
		oauth2Cfg.Endpoint.TokenURL = cfg.TokenEndpoint
	}

	return provider, oauth2Cfg, nil
}

// gcLoop periodically removes expired pending states.
func (p *OIDCProvider) gcLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		p.mu.Lock()
		for k, v := range p.pending {
			if now.After(v.expiry) {
				delete(p.pending, k)
			}
		}
		p.mu.Unlock()
	}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
