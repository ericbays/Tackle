package tracking

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"tackle/internal/crypto"
)

const (
	// tokenRandomBytes is the number of random bytes in the token's random component.
	tokenRandomBytes = 16
	// tokenHMACBytes is the number of HMAC bytes to include in the signature.
	tokenHMACBytes = 8
	// tokenSeparator is the character separating random component from HMAC signature.
	tokenSeparator = "."
)

// TokenService handles generation, embedding, and resolution of tracking tokens.
// Each token is a cryptographically signed identifier that links a capture event
// to a specific target in a campaign.
type TokenService struct {
	hmac *crypto.HMACService
}

// NewTokenService creates a new TokenService using the given HMAC service.
// The HMAC key should be a subkey derived via crypto.DeriveSubkey(masterKey, purpose).
func NewTokenService(hmac *crypto.HMACService) *TokenService {
	return &TokenService{
		hmac: hmac,
	}
}

// GenerateToken creates a new tracking token for a target in a campaign.
// The token is opaque and not reversible to target identity without the HMAC key
// and a database lookup.
//
// Token format: {random_component}.{hmac_signature}
// - Random component: 16 bytes of crypto/rand, base64url-encoded (22 chars)
// - HMAC signature: HMAC-SHA256 of campaign_id + target_id + random_component, truncated to 8 bytes, base64url-encoded (11 chars)
// - Full token: ~34 characters, e.g. "dGhpcyBpcyBhIHRlc3Q.kL9mX2pRzQ"
func (s *TokenService) GenerateToken(campaignID, targetID string) (string, error) {
	// Generate random component (16 bytes = 128 bits of entropy)
	randomBytes := make([]byte, tokenRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate token: random bytes: %w", err)
	}

	// Compute HMAC signature over campaign_id, target_id, and random component
	// Fields are joined with NUL separator to match HMACService.ComputeChecksum format
	hmacInput := append(append(
		[]byte(campaignID),
		0x00,
	),
		append([]byte(targetID), 0x00)...,
	)
	hmacInput = append(hmacInput, randomBytes...)

	// Compute HMAC using the HMAC service's key
	// HMAC service uses NUL-separated fields
	hmacBytes := s.hmac.ComputeChecksum(
		campaignID,
		targetID,
		base64.RawURLEncoding.EncodeToString(randomBytes),
	)

	// Truncate HMAC to desired length (first tokenHMACBytes * 2 because hex encoding)
	if len(hmacBytes) < tokenHMACBytes*2 {
		return "", fmt.Errorf("generate token: hmac too short: got %d, want at least %d", len(hmacBytes), tokenHMACBytes*2)
	}
	signature := hmacBytes[:tokenHMACBytes*2]

	// Base64url encode both components (no padding, URL-safe)
	randomEnc := base64.RawURLEncoding.EncodeToString(randomBytes)

	return fmt.Sprintf("%s%s%s", randomEnc, tokenSeparator, signature), nil
}

// ValidateToken checks that a token's HMAC signature is valid for the given
// campaign and target. This detects tampering but does NOT prove the token
// maps to a real target — that requires a database lookup.
func (s *TokenService) ValidateToken(token, campaignID, targetID string) bool {
	randomComp, signature, err := ParseToken(token)
	if err != nil {
		return false
	}

	// Decode random component from base64url
	randomBytes, err := base64.RawURLEncoding.DecodeString(randomComp)
	if err != nil {
		return false
	}

	// Compute expected signature
	hmacBytes := s.hmac.ComputeChecksum(
		campaignID,
		targetID,
		base64.RawURLEncoding.EncodeToString(randomBytes),
	)
	expectedSignature := hmacBytes[:tokenHMACBytes*2] // first N bytes as hex

	// Compare signatures (case-insensitive hex comparison)
	return strings.EqualFold(signature, expectedSignature)
}

// ParseToken splits a token into its random component and HMAC signature.
// Returns an error if the token format is invalid.
func ParseToken(token string) (random string, signature string, err error) {
	if token == "" {
		return "", "", errors.New("parse token: empty token")
	}

	parts := strings.Split(token, tokenSeparator)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("parse token: invalid format, expected 2 parts separated by '%s', got %d", tokenSeparator, len(parts))
	}

	return parts[0], parts[1], nil
}

// ResolvedToken holds the result of a tracking token lookup.
type ResolvedToken struct {
	CampaignID string
	TargetID   string
	VariantID  string // May be empty if no email record exists yet.
}

// ResolveToken looks up a tracking token in the database and returns the
// associated campaign_id, target_id, and variant_id. Returns a zero value
// with Found=false semantics (empty CampaignID) if not found.
// This is the primary method used by the capture handler.
func (s *TokenService) ResolveToken(ctx context.Context, db *sql.DB, token string) (ResolvedToken, error) {
	// First try campaign_emails which has variant_id.
	row := db.QueryRowContext(ctx, `
		SELECT ce.campaign_id, ce.target_id, COALESCE(ce.variant_id, '')
		FROM campaign_emails ce
		WHERE ce.tracking_token = $1
		LIMIT 1
	`, token)

	var result ResolvedToken
	if err := row.Scan(&result.CampaignID, &result.TargetID, &result.VariantID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return ResolvedToken{}, fmt.Errorf("resolve token: email scan: %w", err)
		}
		// Fall back to campaign_targets (token may exist before emails are queued).
		row2 := db.QueryRowContext(ctx, `
			SELECT campaign_id, target_id
			FROM campaign_targets
			WHERE tracking_token = $1
		`, token)
		var cID, tID sql.NullString
		if err2 := row2.Scan(&cID, &tID); err2 != nil {
			if errors.Is(err2, sql.ErrNoRows) {
				return ResolvedToken{}, nil
			}
			return ResolvedToken{}, fmt.Errorf("resolve token: target scan: %w", err2)
		}
		if cID.Valid {
			result.CampaignID = cID.String
		}
		if tID.Valid {
			result.TargetID = tID.String
		}
	}

	return result, nil
}

// GenerateTokensForCampaign generates tracking tokens for all targets in a campaign
// that don't already have one. Returns the count of tokens generated.
func (s *TokenService) GenerateTokensForCampaign(ctx context.Context, db *sql.DB, campaignID string) (int, error) {
	// Query all targets in campaign without tokens
	query := `
		SELECT target_id
		FROM campaign_targets
		WHERE campaign_id = $1
			AND (tracking_token IS NULL OR tracking_token = '')
	`

	rows, err := db.QueryContext(ctx, query, campaignID)
	if err != nil {
		return 0, fmt.Errorf("generate tokens for campaign: query targets: %w", err)
	}
	defer rows.Close()

	// Track tokens that need to be inserted as a batch
	type tokenData struct {
		targetID string
		token    string
	}

	var tokens []tokenData

	for rows.Next() {
		var targetID string
		if err := rows.Scan(&targetID); err != nil {
			return 0, fmt.Errorf("generate tokens for campaign: scan row: %w", err)
		}

		token, err := s.GenerateToken(campaignID, targetID)
		if err != nil {
			return 0, fmt.Errorf("generate tokens for campaign: generate token for target %s: %w", targetID, err)
		}

		tokens = append(tokens, tokenData{
			targetID: targetID,
			token:    token,
		})
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("generate tokens for campaign: rows error: %w", err)
	}

	if len(tokens) == 0 {
		return 0, nil
	}

	// Batch update tokens using a transaction
	txn, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("generate tokens for campaign: begin transaction: %w", err)
	}
	defer txn.Rollback()

	for _, td := range tokens {
		_, err := txn.ExecContext(ctx, `
			UPDATE campaign_targets
			SET tracking_token = $1
			WHERE campaign_id = $2 AND target_id = $3
		`, td.token, campaignID, td.targetID)
		if err != nil {
			return 0, fmt.Errorf("generate tokens for campaign: update target %s: %w", td.targetID, err)
		}
	}

	if err := txn.Commit(); err != nil {
		return 0, fmt.Errorf("generate tokens for campaign: commit: %w", err)
	}

	return len(tokens), nil
}

// EmbedTokenInURL adds the tracking token as a query parameter to a URL.
// Parameter name is configurable (default "_t").
func EmbedTokenInURL(baseURL, token, paramName string) (string, error) {
	if paramName == "" {
		paramName = "_t"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("embed token in URL: parse base URL: %w", err)
	}

	// Add query parameter
	if u.RawQuery == "" {
		u.RawQuery = paramName + "=" + token
	} else {
		u.RawQuery += "&" + paramName + "=" + token
	}

	return u.String(), nil
}

// TokenBytes is a convenience type alias for the raw random bytes.
type TokenBytes []byte

// RandomBytes returns the random component of a token as bytes.
func (s *TokenService) RandomBytes(token string) ([]byte, error) {
	randomComp, _, err := ParseToken(token)
	if err != nil {
		return nil, err
	}
	return base64.RawURLEncoding.DecodeString(randomComp)
}

// SignatureBytes returns the HMAC signature component of a token as bytes.
func (s *TokenService) SignatureBytes(token string) ([]byte, error) {
	_, signature, err := ParseToken(token)
	if err != nil {
		return nil, err
	}
	// signature is hex-encoded, decode to bytes
	return decodeHex(signature)
}

// decodeHex decodes a hex string to bytes, accepting both upper and lower case.
func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		s = "0" + s // pad if odd length
	}
	return hexDecode(s)
}

// hexDecode is a simple hex decoder using standard library.
func hexDecode(s string) ([]byte, error) {
	// Convert hex string to bytes using encoding/hex
	// This is a simple implementation since we're avoiding external deps
	n := len(s)
	if n%2 != 0 {
		return nil, errors.New("hexDecode: odd length")
	}
	dst := make([]byte, n/2)
	for i := 0; i < n/2; i++ {
		dst[i] = hexByte(s[i*2])<<4 | hexByte(s[i*2+1])
	}
	return dst, nil
}

// hexByte converts a hex character to its byte value.
func hexByte(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return 10 + b - 'a'
	case b >= 'A' && b <= 'F':
		return 10 + b - 'A'
	default:
		return 0xFF // invalid marker
	}
}
