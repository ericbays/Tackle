package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRefreshToken_IssueAndConsume(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewRefreshTokenService(db)
	userID := "user-uuid-1"
	accessHash := "accesshash"

	mock.ExpectExec(`INSERT INTO sessions`).
		WithArgs(userID, accessHash, sqlmock.AnyArg(), "", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rawToken, err := svc.Issue(context.Background(), userID, accessHash, "", "", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Issue error: %v", err)
	}
	if rawToken == "" {
		t.Fatal("expected non-empty raw token")
	}

	// Verify hash is different from raw token.
	hash := hashToken(rawToken)
	if hash == rawToken {
		t.Error("hash must differ from raw token")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRefreshToken_ConsumeRevoked_TriggersRevokeAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewRefreshTokenService(db)
	raw := "some-raw-token"
	hash := hashToken(raw)

	expiry := time.Now().Add(7 * 24 * time.Hour)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "token_hash", "refresh_token_hash",
		"ip_address", "user_agent", "expires_at", "last_used_at", "revoked", "created_at",
	}).AddRow("sess-1", "user-1", "accesshash", hash, "", "", expiry, nil, true, time.Now())

	mock.ExpectQuery(`SELECT id`).WithArgs(hash).WillReturnRows(rows)
	mock.ExpectExec(`UPDATE sessions SET revoked`).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 2))

	_, err = svc.Consume(context.Background(), raw)
	if err != ErrTokenReuse {
		t.Fatalf("expected ErrTokenReuse, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRefreshToken_ConsumeNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewRefreshTokenService(db)
	rows := sqlmock.NewRows([]string{"id"}) // empty
	mock.ExpectQuery(`SELECT id`).WithArgs(sqlmock.AnyArg()).WillReturnRows(rows)

	_, err = svc.Consume(context.Background(), "nonexistent-token")
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	t1, err1 := generateToken()
	t2, err2 := generateToken()
	if err1 != nil || err2 != nil {
		t.Fatalf("generateToken errors: %v, %v", err1, err2)
	}
	if t1 == t2 {
		t.Error("expected unique tokens, got identical values")
	}
}
