package auth

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"tackle/internal/models"
)

const jwtIssuer = "tackle"

// Claims is the JWT payload embedded in every Tackle access token.
type Claims struct {
	jwt.RegisteredClaims
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	JTI         string   `json:"jti"`
}

// JWTService issues and validates Tackle access tokens.
// Supports HS256 (default) and RS256 signing methods.
type JWTService struct {
	hmacKey         []byte
	rsaPrivateKey   *rsa.PrivateKey
	rsaPublicKey    *rsa.PublicKey
	signingMethod   jwt.SigningMethod
	lifetimeMinutes int
}

// NewJWTService creates a JWTService using HMAC-SHA256 signing.
// lifetimeMinutes controls how long issued tokens are valid.
func NewJWTService(signingKey []byte, lifetimeMinutes int) *JWTService {
	return &JWTService{
		hmacKey:         signingKey,
		signingMethod:   jwt.SigningMethodHS256,
		lifetimeMinutes: lifetimeMinutes,
	}
}

// NewJWTServiceRS256 creates a JWTService using RSA-SHA256 signing.
// The private key is used for signing; the public key is derived from it for verification.
func NewJWTServiceRS256(privateKey *rsa.PrivateKey, lifetimeMinutes int) *JWTService {
	return &JWTService{
		rsaPrivateKey:   privateKey,
		rsaPublicKey:    &privateKey.PublicKey,
		signingMethod:   jwt.SigningMethodRS256,
		lifetimeMinutes: lifetimeMinutes,
	}
}

// Issue creates and signs a JWT access token for the given user, role, and permissions.
// A unique JTI (UUID v4) is generated per token.
func (j *JWTService) Issue(user *models.User, role string, permissions []string) (string, error) {
	return j.IssueWithExpiry(user.ID, user.Username, user.Email, role, permissions, time.Duration(j.lifetimeMinutes)*time.Minute)
}

// IssueWithExpiry creates a JWT access token with the specified expiry duration.
func (j *JWTService) IssueWithExpiry(userID, username, email, role string, permissions []string, expiry time.Duration) (string, error) {
	jti := uuid.New().String()
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			Issuer:    jwtIssuer,
		},
		Username:    username,
		Email:       email,
		Role:        role,
		Permissions: permissions,
		JTI:         jti,
	}
	token := jwt.NewWithClaims(j.signingMethod, claims)
	signed, err := token.SignedString(j.signingKey())
	if err != nil {
		return "", fmt.Errorf("jwt issue: sign: %w", err)
	}
	return signed, nil
}

// IssueExternal creates and signs a JWT access token using explicit field values
// rather than a models.User struct. Used for externally-provisioned users.
func (j *JWTService) IssueExternal(userID, username, email, role string, permissions []string) (string, error) {
	return j.IssueWithExpiry(userID, username, email, role, permissions, time.Duration(j.lifetimeMinutes)*time.Minute)
}

// Validate parses and validates a signed JWT string.
// Returns the embedded Claims on success.
// Returns an error if the token is expired, has an invalid signature, or has wrong issuer.
func (j *JWTService) Validate(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != j.signingMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.verifyKey(), nil
	}, jwt.WithIssuer(jwtIssuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("jwt validate: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwt validate: invalid claims")
	}
	return claims, nil
}

// signingKey returns the key used for signing tokens.
func (j *JWTService) signingKey() any {
	if j.rsaPrivateKey != nil {
		return j.rsaPrivateKey
	}
	return j.hmacKey
}

// verifyKey returns the key used for verifying tokens.
func (j *JWTService) verifyKey() any {
	if j.rsaPublicKey != nil {
		return j.rsaPublicKey
	}
	return j.hmacKey
}
