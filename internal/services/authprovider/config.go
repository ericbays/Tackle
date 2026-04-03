// Package authprovider implements OIDC, FusionAuth, and LDAP external authentication providers.
package authprovider

// OIDCConfig holds the configuration for a generic OIDC provider.
type OIDCConfig struct {
	IssuerURL             string   `json:"issuer_url"`
	ClientID              string   `json:"client_id"`
	ClientSecret          string   `json:"client_secret"`
	Scopes                []string `json:"scopes"`
	RedirectURI           string   `json:"redirect_uri"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint,omitempty"`
	TokenEndpoint         string   `json:"token_endpoint,omitempty"`
	AuthorizationEndpoint string   `json:"authorization_endpoint,omitempty"`
}

// FusionAuthConfig holds the configuration for a FusionAuth provider.
type FusionAuthConfig struct {
	BaseURL       string `json:"base_url"`
	ApplicationID string `json:"application_id"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	TenantID      string `json:"tenant_id,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	RedirectURI   string `json:"redirect_uri"`
}

// LDAPConfig holds the configuration for an LDAP / Active Directory provider.
type LDAPConfig struct {
	ServerURLs        []string          `json:"server_urls"`
	BindDN            string            `json:"bind_dn"`
	BindPassword      string            `json:"bind_password"`
	BaseDN            string            `json:"base_dn"`
	UserSearchFilter  string            `json:"user_search_filter"`
	AttributeMappings LDAPAttrMappings  `json:"attribute_mappings"`
	GroupBaseDN       string            `json:"group_base_dn,omitempty"`
	GroupSearchFilter string            `json:"group_search_filter,omitempty"`
	StartTLS          bool              `json:"start_tls"`
	SkipCertVerify    bool              `json:"skip_cert_verify"`
}

// LDAPAttrMappings maps LDAP attribute names to Tackle user fields.
type LDAPAttrMappings struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	DisplayName     string `json:"display_name"`
	GroupMembership string `json:"group_membership"`
}

// ExternalClaims is the normalized set of claims extracted from any external provider.
type ExternalClaims struct {
	// Subject is the provider-unique user identifier (OIDC sub, LDAP DN, etc.).
	Subject  string
	Email    string
	Name     string
	Username string
	Groups   []string
}

// TestResult holds the outcome of a provider connectivity test.
type TestResult struct {
	Success     bool
	StageReached string
	ErrorDetail string
}
