package rbac

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestResolver_AdminResolvesToAllPermissions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT ro.name`).
		WithArgs("user-admin").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("admin"))

	r := NewResolver(db)
	perms, err := r.Resolve(context.Background(), "user-admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) != len(Registry) {
		t.Errorf("admin should resolve to all %d permissions, got %d", len(Registry), len(perms))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResolver_NonAdminResolvesToRolePermissions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT ro.name`).
		WithArgs("user-engineer").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("engineer"))

	permRows := sqlmock.NewRows([]string{"perm"}).
		AddRow("campaigns:read").
		AddRow("endpoints:create")
	mock.ExpectQuery(`SELECT p.resource_type`).
		WithArgs("user-engineer").
		WillReturnRows(permRows)

	r := NewResolver(db)
	perms, err := r.Resolve(context.Background(), "user-engineer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(perms))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResolver_RoleForUser_NoRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT ro.name`).
		WithArgs("user-none").
		WillReturnRows(sqlmock.NewRows([]string{"name"}))

	r := NewResolver(db)
	_, err = r.RoleForUser(context.Background(), "user-none")
	if err == nil {
		t.Error("expected error when no role assigned")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
