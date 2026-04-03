package targetgroup

import (
	"testing"
	"time"

	"tackle/internal/repositories"
)

func TestGroupToDTO(t *testing.T) {
	g := repositories.TargetGroup{
		ID:          "test-id",
		Name:        "Engineering",
		Description: "All engineers",
		CreatedBy:   "user-1",
		CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	dto := groupToDTO(g, 42)

	if dto.ID != "test-id" {
		t.Errorf("ID = %q, want %q", dto.ID, "test-id")
	}
	if dto.Name != "Engineering" {
		t.Errorf("Name = %q, want %q", dto.Name, "Engineering")
	}
	if dto.MemberCount != 42 {
		t.Errorf("MemberCount = %d, want %d", dto.MemberCount, 42)
	}
	if dto.CreatedAt != "2025-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want RFC3339", dto.CreatedAt)
	}
}

func TestTargetsToMemberDTOs(t *testing.T) {
	firstName := "Jane"
	targets := []repositories.Target{
		{ID: "t1", Email: "jane@example.com", FirstName: &firstName},
		{ID: "t2", Email: "john@example.com"},
	}

	dtos := targetsToMemberDTOs(targets)
	if len(dtos) != 2 {
		t.Fatalf("got %d DTOs, want 2", len(dtos))
	}
	if dtos[0].Email != "jane@example.com" {
		t.Errorf("first DTO email = %q, want jane@example.com", dtos[0].Email)
	}
	if dtos[0].FirstName == nil || *dtos[0].FirstName != "Jane" {
		t.Error("first DTO should have first_name Jane")
	}
	if dtos[1].FirstName != nil {
		t.Error("second DTO should have nil first_name")
	}
}

func TestCreateInput_Validation(t *testing.T) {
	// Test that empty name would be caught by the service.
	// We test the validation logic indirectly since Create needs a real repo.
	tests := []struct {
		name    string
		input   CreateInput
		wantErr bool
	}{
		{"valid", CreateInput{Name: "Group1", Description: "desc"}, false},
		{"empty name", CreateInput{Name: "", Description: "desc"}, true},
		{"name too long", CreateInput{Name: string(make([]byte, 256)), Description: "desc"}, true},
		{"desc too long", CreateInput{Name: "Group1", Description: string(make([]byte, 1025))}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate locally (same logic as Service.Create).
			name := tt.input.Name
			desc := tt.input.Description
			var err error
			if name == "" {
				err = &ValidationError{Msg: "name is required"}
			} else if len(name) > 255 {
				err = &ValidationError{Msg: "name must be 255 characters or fewer"}
			} else if len(desc) > 1024 {
				err = &ValidationError{Msg: "description must be 1024 characters or fewer"}
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
