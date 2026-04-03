package authproviders

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/repositories"
	"tackle/internal/services/authprovider"
	authsvc "tackle/internal/services/auth"
	"tackle/pkg/response"
)

// --- Provider Configuration (Admin) ---

// ListProviders handles GET /api/v1/settings/auth-providers.
func (d *Deps) ListProviders(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	providers, err := d.Svc.ListProviderConfigs(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list auth providers", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, providers)
}

// GetProvider handles GET /api/v1/settings/auth-providers/{id}.
func (d *Deps) GetProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	p, err := d.Svc.GetProviderConfig(r.Context(), id)
	if err != nil {
		response.Error(w, "NOT_FOUND", "auth provider not found", http.StatusNotFound, correlationID)
		return
	}
	response.Success(w, p)
}

// CreateProvider handles POST /api/v1/settings/auth-providers.
func (d *Deps) CreateProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	var input authprovider.ProviderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	p, err := d.Svc.CreateProviderConfig(r.Context(), actorID, input)
	if err != nil {
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	response.Created(w, p)
}

// UpdateProvider handles PUT /api/v1/settings/auth-providers/{id}.
func (d *Deps) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	var input authprovider.ProviderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	p, err := d.Svc.UpdateProviderConfig(r.Context(), actorID, id, input)
	if err != nil {
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	response.Success(w, p)
}

// DeleteProvider handles DELETE /api/v1/settings/auth-providers/{id}.
func (d *Deps) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	if err := d.Svc.DeleteProviderConfig(r.Context(), actorID, id); err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestProvider handles POST /api/v1/settings/auth-providers/{id}/test.
func (d *Deps) TestProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	result, err := d.Svc.TestProviderConfig(r.Context(), actorID, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// EnableProvider handles POST /api/v1/settings/auth-providers/{id}/enable.
func (d *Deps) EnableProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	if err := d.Svc.EnableProviderConfig(r.Context(), actorID, id); err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DisableProvider handles POST /api/v1/settings/auth-providers/{id}/disable.
func (d *Deps) DisableProvider(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	if err := d.Svc.DisableProviderConfig(r.Context(), actorID, id); err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetRoleMappings handles PUT /api/v1/settings/auth-providers/{id}/role-mappings.
func (d *Deps) SetRoleMappings(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	var input []struct {
		ExternalGroup string `json:"external_group"`
		RoleID        string `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	mappings := make([]repositories.RoleMapping, 0, len(input))
	for _, m := range input {
		if m.ExternalGroup == "" || m.RoleID == "" {
			response.Error(w, "BAD_REQUEST", "external_group and role_id are required", http.StatusBadRequest, correlationID)
			return
		}
		mappings = append(mappings, repositories.RoleMapping{
			ProviderConfigID: id,
			ExternalGroup:    m.ExternalGroup,
			RoleID:           m.RoleID,
		})
	}
	claims := middleware.ClaimsFromContext(r.Context())
	actorID := ""
	if claims != nil {
		actorID = claims.Subject
	}
	if err := d.Svc.SetRoleMappings(r.Context(), actorID, id, mappings); err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetRoleMappings handles GET /api/v1/settings/auth-providers/{id}/role-mappings.
func (d *Deps) GetRoleMappings(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	mappings, err := d.Svc.GetRoleMappings(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, mappings)
}

// --- Public Auth Flow ---

// GetEnabledProviders handles GET /api/v1/auth/providers.
// Returns enabled external providers for the login page (no secrets).
func (d *Deps) GetEnabledProviders(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	providers, err := d.Svc.GetEnabledProviders(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list providers", http.StatusInternalServerError, correlationID)
		return
	}
	// Always include local provider first.
	all := append([]authprovider.AuthProviderSummary{
		{ID: "local", Type: "local", Name: "Local Account", ButtonLabel: "Sign in with password"},
	}, providers...)
	response.Success(w, all)
}

// OIDCLogin handles GET /api/v1/auth/oidc/{providerID}/login.
// Initiates the OIDC/FusionAuth Authorization Code Flow redirect.
func (d *Deps) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	providerID := chi.URLParam(r, "providerID")

	redirectURL, _, err := d.Svc.InitiateOIDCFlow(r.Context(), providerID, false, "")
	if err != nil {
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OIDCCallback handles GET /api/v1/auth/oidc/callback/{providerID}.
// Completes the OIDC/FusionAuth Authorization Code Flow.
func (d *Deps) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	providerID := chi.URLParam(r, "providerID")

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		response.Error(w, "BAD_REQUEST", "missing code or state", http.StatusBadRequest, correlationID)
		return
	}

	provisioned, err := d.Svc.HandleOIDCCallback(r.Context(), providerID, code, state)
	if err != nil {
		response.Error(w, "UNAUTHORIZED", "authentication failed", http.StatusUnauthorized, correlationID)
		return
	}

	if provisioned.NeedsLinking {
		// Redirect to frontend with link-required signal.
		redirectURL := fmt.Sprintf("/auth/callback?error=ACCOUNT_LINK_REQUIRED&provider_id=%s&email=%s",
			providerID, provisioned.Email)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	tokenResp, err := d.issueTokens(w, r, provisioned)
	if err != nil {
		// Redirect to frontend with error.
		http.Redirect(w, r, "/auth/callback?error=TOKEN_ISSUE_FAILED", http.StatusFound)
		return
	}

	// Redirect to frontend with tokens via URL fragment (not query params for security).
	redirectURL := fmt.Sprintf("/auth/callback?access_token=%s&provider=%s",
		tokenResp.AccessToken, "external")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// --- Account Linking (Authenticated) ---

// InitiateLink handles POST /api/v1/auth/link/{providerID}.
// Starts the external auth flow for account linking.
func (d *Deps) InitiateLink(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	providerID := chi.URLParam(r, "providerID")
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	redirectURL, _, err := d.Svc.InitiateOIDCFlow(r.Context(), providerID, true, claims.Subject)
	if err != nil {
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	response.Success(w, map[string]string{"redirect_url": redirectURL})
}

// LinkCallback handles GET /api/v1/auth/link/callback/{providerID}.
// Completes the account linking flow.
func (d *Deps) LinkCallback(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	providerID := chi.URLParam(r, "providerID")

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		response.Error(w, "BAD_REQUEST", "missing code or state", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Svc.HandleLinkCallback(r.Context(), providerID, code, state); err != nil {
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnlinkIdentity handles DELETE /api/v1/auth/identities/{identityID}.
func (d *Deps) UnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	identityID := chi.URLParam(r, "identityID")
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	if err := d.LinkSvc.UnlinkIdentity(r.Context(), claims.Subject, identityID); err != nil {
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListIdentities handles GET /api/v1/auth/identities.
func (d *Deps) ListIdentities(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	identities, err := d.LinkSvc.GetLinkedIdentities(r.Context(), claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, identities)
}

// --- helpers ---

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID          string   `json:"id"`
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Roles       []string `json:"roles"`
		Permissions []string `json:"permissions"`
	} `json:"user"`
}

func (d *Deps) issueTokens(w http.ResponseWriter, r *http.Request, user authprovider.ProvisionedUser) (loginResponse, error) {
	accessToken, err := d.JWTSvc.IssueExternal(user.UserID, user.Username, user.Email, user.RoleName, user.Permissions)
	if err != nil {
		return loginResponse{}, err
	}

	accessHash := authsvc.HashTokenPublic(accessToken)
	rawRefresh, err := d.RefreshSvc.Issue(r.Context(), user.UserID, accessHash, r.RemoteAddr, r.UserAgent(), 7*24*time.Hour)
	if err != nil {
		return loginResponse{}, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rawRefresh,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	var resp loginResponse
	resp.AccessToken = accessToken
	resp.RefreshToken = rawRefresh
	resp.TokenType = "Bearer"
	resp.ExpiresIn = 900
	resp.User.ID = user.UserID
	resp.User.Username = user.Username
	resp.User.DisplayName = user.DisplayName
	resp.User.Roles = []string{user.RoleName}
	resp.User.Permissions = user.Permissions
	return resp, nil
}
