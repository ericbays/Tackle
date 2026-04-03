package auth

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func hashForTest(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return string(h)
}

func TestHistoryChecker_IsReused_Match(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	oldHash := hashForTest(t, "OldPassword1!")
	rows := sqlmock.NewRows([]string{"password_hash"}).AddRow(oldHash)
	mock.ExpectQuery(`SELECT password_hash`).
		WithArgs("user-1", 5).
		WillReturnRows(rows)

	hc := NewHistoryChecker(db)
	reused, err := hc.IsReused(context.Background(), "user-1", "OldPassword1!", 5)
	if err != nil {
		t.Fatalf("IsReused error: %v", err)
	}
	if !reused {
		t.Error("expected reused=true for matching old password")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHistoryChecker_IsReused_NoMatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	oldHash := hashForTest(t, "OldPassword1!")
	rows := sqlmock.NewRows([]string{"password_hash"}).AddRow(oldHash)
	mock.ExpectQuery(`SELECT password_hash`).
		WithArgs("user-1", 5).
		WillReturnRows(rows)

	hc := NewHistoryChecker(db)
	reused, err := hc.IsReused(context.Background(), "user-1", "CompletelyDifferent9!", 5)
	if err != nil {
		t.Fatalf("IsReused error: %v", err)
	}
	if reused {
		t.Error("expected reused=false for non-matching password")
	}
}

func TestHistoryChecker_Record(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`INSERT INTO password_history`).
		WithArgs("user-1", "somehash").
		WillReturnResult(sqlmock.NewResult(1, 1))

	hc := NewHistoryChecker(db)
	if err := hc.Record(context.Background(), "user-1", "somehash"); err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
