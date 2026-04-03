package authprovider

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
)

// LDAPProvider handles LDAP bind authentication.
type LDAPProvider struct{}

// NewLDAPProvider creates an LDAPProvider.
func NewLDAPProvider() *LDAPProvider { return &LDAPProvider{} }

// Authenticate performs an LDAP bind authentication for the given username and password.
// Returns normalized ExternalClaims on success.
func (p *LDAPProvider) Authenticate(ctx context.Context, cfg LDAPConfig, username, password string) (ExternalClaims, error) {
	if password == "" {
		return ExternalClaims{}, fmt.Errorf("ldap authenticate: empty password rejected")
	}

	conn, err := p.dial(cfg)
	if err != nil {
		return ExternalClaims{}, fmt.Errorf("ldap authenticate: dial: %w", err)
	}
	defer conn.Close()

	// Bind as service account to search for the user.
	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return ExternalClaims{}, fmt.Errorf("ldap authenticate: service bind: %w", err)
	}

	// Build and run user search.
	filter := strings.ReplaceAll(cfg.UserSearchFilter, "{{username}}", ldap.EscapeFilter(username))
	attrs := []string{
		"dn",
		cfg.AttributeMappings.Email,
		cfg.AttributeMappings.DisplayName,
		cfg.AttributeMappings.Username,
		cfg.AttributeMappings.GroupMembership,
	}
	// Deduplicate and remove empty attr names.
	attrs = cleanAttrs(attrs)

	searchReq := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // size limit
		int(10*time.Second/time.Second),
		false,
		filter,
		attrs,
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		// Generic 401 — do not enumerate users.
		return ExternalClaims{}, fmt.Errorf("ldap authenticate: user not found")
	}
	if len(result.Entries) == 0 {
		return ExternalClaims{}, fmt.Errorf("ldap authenticate: user not found")
	}
	entry := result.Entries[0]
	userDN := entry.DN

	// Attempt bind as the found user to verify the password.
	if err := conn.Bind(userDN, password); err != nil {
		// Still generic — don't distinguish "wrong password" vs "account locked".
		return ExternalClaims{}, fmt.Errorf("ldap authenticate: invalid credentials")
	}

	// Extract attributes.
	email := entry.GetAttributeValue(cfg.AttributeMappings.Email)
	displayName := entry.GetAttributeValue(cfg.AttributeMappings.DisplayName)
	attrUsername := entry.GetAttributeValue(cfg.AttributeMappings.Username)
	memberOf := entry.GetAttributeValues(cfg.AttributeMappings.GroupMembership)

	// Optionally fetch additional groups via group search.
	groups := p.extractGroups(memberOf)
	if cfg.GroupBaseDN != "" && cfg.GroupSearchFilter != "" {
		// Re-bind as service account to do group search.
		if err2 := conn.Bind(cfg.BindDN, cfg.BindPassword); err2 == nil {
			extra := p.searchGroups(conn, cfg, userDN)
			groups = appendUnique(groups, extra...)
		}
	}

	if attrUsername == "" {
		attrUsername = username
	}
	return ExternalClaims{
		Subject:  userDN,
		Email:    email,
		Name:     displayName,
		Username: attrUsername,
		Groups:   groups,
	}, nil
}

// TestConnection verifies LDAP connectivity and service-account bind.
func (p *LDAPProvider) TestConnection(ctx context.Context, cfg LDAPConfig) TestResult {
	conn, err := p.dial(cfg)
	if err != nil {
		return TestResult{StageReached: "dial", ErrorDetail: err.Error()}
	}
	defer conn.Close()

	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return TestResult{StageReached: "bind", ErrorDetail: err.Error()}
	}

	// Verify base DN is accessible with a base-scope search.
	sr := ldap.NewSearchRequest(cfg.BaseDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases,
		1, 5, false, "(objectClass=*)", []string{"dn"}, nil)
	if _, err := conn.Search(sr); err != nil {
		return TestResult{StageReached: "base_dn_check", ErrorDetail: err.Error()}
	}

	return TestResult{Success: true, StageReached: "base_dn_check"}
}

func (p *LDAPProvider) dial(cfg LDAPConfig) (*ldap.Conn, error) {
	if len(cfg.ServerURLs) == 0 {
		return nil, fmt.Errorf("no server URLs configured")
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.SkipCertVerify, //nolint:gosec // lab env option
	}

	var (
		conn *ldap.Conn
		err  error
	)
	for _, url := range cfg.ServerURLs {
		if strings.HasPrefix(url, "ldaps://") {
			conn, err = ldap.DialURL(url, ldap.DialWithTLSConfig(tlsCfg))
		} else {
			conn, err = ldap.DialURL(url)
			if err == nil && cfg.StartTLS {
				if tlsErr := conn.StartTLS(tlsCfg); tlsErr != nil {
					conn.Close()
					err = tlsErr
					conn = nil
					continue
				}
			}
		}
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("could not connect to any LDAP server: %w", err)
}

// extractGroups parses the memberOf attribute values into simple CN names.
func (p *LDAPProvider) extractGroups(memberOf []string) []string {
	out := make([]string, 0, len(memberOf))
	for _, dn := range memberOf {
		// Extract the CN from the DN, e.g. "CN=GroupName,DC=..."
		for _, part := range strings.Split(dn, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 && strings.EqualFold(kv[0], "cn") {
				out = append(out, kv[1])
				break
			}
		}
	}
	return out
}

// searchGroups performs a group search to find groups that have the user as a member.
func (p *LDAPProvider) searchGroups(conn *ldap.Conn, cfg LDAPConfig, userDN string) []string {
	filter := strings.ReplaceAll(cfg.GroupSearchFilter, "{{dn}}", ldap.EscapeFilter(userDN))
	sr := ldap.NewSearchRequest(
		cfg.GroupBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, 10, false, filter, []string{"cn"}, nil,
	)
	result, err := conn.Search(sr)
	if err != nil {
		return nil
	}
	var groups []string
	for _, e := range result.Entries {
		if cn := e.GetAttributeValue("cn"); cn != "" {
			groups = append(groups, cn)
		}
	}
	return groups
}

func cleanAttrs(attrs []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(attrs))
	for _, a := range attrs {
		if a != "" && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

func appendUnique(existing []string, items ...string) []string {
	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[e] = true
	}
	for _, item := range items {
		if !seen[item] {
			existing = append(existing, item)
			seen[item] = true
		}
	}
	return existing
}
