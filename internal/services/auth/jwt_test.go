package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"tackle/internal/models"
)

var testUser = &models.User{
	ID:       "user-uuid-1234",
	Username: "testuser",
	Email:    "testuser@example.com",
}

func newTestJWTService(lifetimeMinutes int) *JWTService {
	return NewJWTService([]byte("test-signing-key-32-bytes-padded!"), lifetimeMinutes)
}

func TestJWT_IssueAndValidate(t *testing.T) {
	svc := newTestJWTService(15)
	perms := []string{"campaigns:read", "targets:read"}

	tokenStr, err := svc.Issue(testUser, "operator", perms)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}

	claims, err := svc.Validate(tokenStr)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	if claims.Subject != testUser.ID {
		t.Errorf("sub = %q, want %q", claims.Subject, testUser.ID)
	}
	if claims.Username != testUser.Username {
		t.Errorf("username = %q, want %q", claims.Username, testUser.Username)
	}
	if claims.Email != testUser.Email {
		t.Errorf("email = %q, want %q", claims.Email, testUser.Email)
	}
	if claims.Role != "operator" {
		t.Errorf("role = %q, want %q", claims.Role, "operator")
	}
	if claims.JTI == "" {
		t.Error("expected non-empty JTI")
	}
	if len(claims.Permissions) != len(perms) {
		t.Errorf("permissions count = %d, want %d", len(claims.Permissions), len(perms))
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	svc := newTestJWTService(-1) // already expired
	tokenStr, err := svc.Issue(testUser, "operator", nil)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}
	_, err = svc.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestJWT_WrongSigningKey(t *testing.T) {
	svc := newTestJWTService(15)
	tokenStr, err := svc.Issue(testUser, "operator", nil)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}

	wrongSvc := NewJWTService([]byte("different-signing-key-32-bytes!!"), 15)
	_, err = wrongSvc.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong signing key, got nil")
	}
}

func TestJWT_TamperedToken(t *testing.T) {
	svc := newTestJWTService(15)
	tokenStr, err := svc.Issue(testUser, "operator", nil)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}

	tampered := tokenStr + "x"
	_, err = svc.Validate(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestJWT_RS256_IssueAndValidate(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	svc := NewJWTServiceRS256(key, 15)
	perms := []string{"campaigns:read"}

	tokenStr, err := svc.Issue(testUser, "admin", perms)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}

	claims, err := svc.Validate(tokenStr)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if claims.Subject != testUser.ID {
		t.Errorf("sub = %q, want %q", claims.Subject, testUser.ID)
	}
	if claims.Role != "admin" {
		t.Errorf("role = %q, want %q", claims.Role, "admin")
	}
}

func TestJWT_RS256_WrongKey(t *testing.T) {
	key1, _ := rsa.GenerateKey(rand.Reader, 2048)
	key2, _ := rsa.GenerateKey(rand.Reader, 2048)

	svc1 := NewJWTServiceRS256(key1, 15)
	svc2 := NewJWTServiceRS256(key2, 15)

	tokenStr, err := svc1.Issue(testUser, "operator", nil)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}
	_, err = svc2.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong RSA key, got nil")
	}
}

func TestJWT_RS256_RejectsHS256Token(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	rsaSvc := NewJWTServiceRS256(rsaKey, 15)

	hmacSvc := newTestJWTService(15)
	tokenStr, _ := hmacSvc.Issue(testUser, "operator", nil)

	_, err := rsaSvc.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected RS256 service to reject HS256 token")
	}
}

func TestJWT_HS256_RejectsRS256Token(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	rsaSvc := NewJWTServiceRS256(rsaKey, 15)

	tokenStr, _ := rsaSvc.Issue(testUser, "operator", nil)

	hmacSvc := newTestJWTService(15)
	_, err := hmacSvc.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected HS256 service to reject RS256 token")
	}
}

func TestJWT_WrongAlgorithm(t *testing.T) {
	// Craft a token with RS256 method header manually — Validate must reject it.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "x",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			Issuer:    jwtIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := token.SignedString([]byte("test-signing-key-32-bytes-padded!"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	svc := newTestJWTService(15)
	_, err = svc.Validate(signed)
	if err == nil {
		t.Fatal("expected error for unexpected signing method, got nil")
	}
}
