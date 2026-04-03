package domain

import (
	"testing"
	"time"

	"tackle/internal/repositories"
)

// TestValidateDomainName covers FQDN validation logic.
func TestValidateDomainName(t *testing.T) {
	cases := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid simple", "example.com", false},
		{"valid subdomain", "sub.example.com", false},
		{"valid multi-label", "a.b.c.example.co.uk", false},
		{"empty", "", true},
		{"no TLD", "example", true},
		{"trailing dot", "example.com.", true},
		{"leading dot", ".example.com", true},
		{"double dot", "example..com", true},
		{"single char TLD", "example.c", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDomainName(tc.domain)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for domain %q, got nil", tc.domain)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for domain %q: %v", tc.domain, err)
			}
		})
	}
}

// TestValidationError confirms the error type satisfies the error interface.
func TestValidationError(t *testing.T) {
	err := &ValidationError{Field: "domain_name", Message: "required"}
	if err.Error() == "" {
		t.Error("ValidationError.Error() should not be empty")
	}
}

// TestConflictError confirms the error type satisfies the error interface.
func TestConflictError(t *testing.T) {
	err := &ConflictError{Message: "domain has active campaign associations"}
	if err.Error() == "" {
		t.Error("ConflictError.Error() should not be empty")
	}
}

// TestToDTO_NilOptionals confirms DTO conversion with no expiry/registration dates doesn't panic.
func TestToDTO_NilOptionals(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("toDTO panicked: %v", r)
		}
	}()
	p := makeTestProfile()
	dto := toDTO(p, 0)
	if dto.ExpiryDate != nil {
		t.Error("expected nil ExpiryDate")
	}
	if dto.RegistrationDate != nil {
		t.Error("expected nil RegistrationDate")
	}
	if dto.Tags == nil {
		t.Error("Tags should never be nil in DTO")
	}
}

// TestToDTO_WithDates confirms tags and campaign count are passed through.
func TestToDTO_WithDates(t *testing.T) {
	p := makeTestProfile()
	p.Tags = []string{"red-team", "phishing"}
	expiry := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	p.ExpiryDate = &expiry

	dto := toDTO(p, 3)
	if dto.CampaignCount != 3 {
		t.Errorf("expected CampaignCount=3, got %d", dto.CampaignCount)
	}
	if len(dto.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(dto.Tags))
	}
	if dto.ExpiryDate == nil {
		t.Error("expected non-nil ExpiryDate")
	}
	if *dto.ExpiryDate != "2026-01-15" {
		t.Errorf("expected ExpiryDate=2026-01-15, got %q", *dto.ExpiryDate)
	}
}

// TestToRenewalDTO confirms renewal record DTO conversion.
func TestToRenewalDTO(t *testing.T) {
	amount := 12.50
	currency := "USD"
	rec := repositories.DomainRenewalRecord{
		ID:              "rec-1",
		DomainProfileID: "dom-1",
		RenewalDate:     time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		DurationYears:   1,
		CostAmount:      &amount,
		CostCurrency:    &currency,
		CreatedAt:       time.Now(),
	}
	dto := toRenewalDTO(rec)
	if dto.RenewalDate != "2026-01-15" {
		t.Errorf("expected RenewalDate=2026-01-15, got %q", dto.RenewalDate)
	}
	if dto.CostAmount == nil || *dto.CostAmount != 12.50 {
		t.Error("expected CostAmount=12.50")
	}
	if dto.CostCurrency == nil || *dto.CostCurrency != "USD" {
		t.Error("expected CostCurrency=USD")
	}
}

func makeTestProfile() repositories.DomainProfile {
	return repositories.DomainProfile{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		DomainName: "example.com",
		Status:     repositories.DomainStatusActive,
		Tags:       []string{},
		CreatedBy:  "user-1",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
