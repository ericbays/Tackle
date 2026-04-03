package auth

import "testing"

func TestPasswordPolicy_Validate(t *testing.T) {
	p := DefaultPolicy()

	tests := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"valid", "Correct!Horse9", false},
		{"too short", "Ab1!", true},
		{"no upper", "correct!horse9", true},
		{"no lower", "CORRECT!HORSE9", true},
		{"no digit", "Correct!Horse", true},
		{"no special", "CorrectHorse9", true},
		{"exactly min length", "Abcdefgh1!ab", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := p.Validate(tc.pw)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tc.pw, err, tc.wantErr)
			}
		})
	}
}

func TestIsBreached(t *testing.T) {
	// "password" and "123456" are in the embedded list.
	if !IsBreached("password") {
		t.Error("expected 'password' to be breached")
	}
	if !IsBreached("PASSWORD") {
		t.Error("expected 'PASSWORD' (case-insensitive) to be breached")
	}
	if !IsBreached("123456") {
		t.Error("expected '123456' to be breached")
	}
	if IsBreached("xK9#mR2!qLpZ77") {
		t.Error("expected random strong password to not be breached")
	}
}

func TestBreachedSetSize(t *testing.T) {
	if len(breachedSet) < 10000 {
		t.Errorf("breached password set has %d entries, want >= 10000", len(breachedSet))
	}
}
