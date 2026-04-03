package auth

import "testing"

func TestHashAndComparePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"simple", "correcthorsebatterystaple"},
		{"complex", "P@ssw0rd!123XYZ"},
		{"unicode", "pässwörð!1A"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := HashPassword(tc.password)
			if err != nil {
				t.Fatalf("HashPassword error: %v", err)
			}
			if hash == "" {
				t.Fatal("expected non-empty hash")
			}
			if err := ComparePassword(hash, tc.password); err != nil {
				t.Fatalf("ComparePassword mismatch on correct password: %v", err)
			}
		})
	}
}

func TestComparePassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if err := ComparePassword(hash, "wrongpassword"); err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	h1, _ := HashPassword("samepassword")
	h2, _ := HashPassword("samepassword")
	if h1 == h2 {
		t.Fatal("expected bcrypt to produce different hashes due to random salt")
	}
}
