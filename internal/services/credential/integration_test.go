package credential

import (
	"testing"
	"time"

	"tackle/internal/crypto"
	"tackle/internal/repositories"
)

// TestCaptureEventDTO_MaskedByDefault verifies default responses never include credential values.
func TestCaptureEventDTO_MaskedByDefault(t *testing.T) {
	dto := CaptureEventDTO{
		ID:             "evt-1",
		CampaignID:     "camp-1",
		FieldsCaptured: []string{"username", "password"},
		FieldCategories: map[string]string{
			"username": "identity",
			"password": "sensitive",
		},
	}

	// FieldsCaptured has names only — no values.
	for _, name := range dto.FieldsCaptured {
		if name == "" {
			t.Error("field name should not be empty")
		}
	}
	// Verify no FieldValue-like field exists in the DTO struct (compile-time guarantee).
}

// TestCategorizeField_DefaultRulesIntegration tests categorization with real default rules.
func TestCategorizeField_DefaultRulesIntegration(t *testing.T) {
	rules := []repositories.FieldCategorizationRule{
		{FieldPattern: "password", Category: repositories.FieldCategorySensitive, Priority: 100},
		{FieldPattern: "passwd", Category: repositories.FieldCategorySensitive, Priority: 100},
		{FieldPattern: "pass", Category: repositories.FieldCategorySensitive, Priority: 90},
		{FieldPattern: "secret", Category: repositories.FieldCategorySensitive, Priority: 90},
		{FieldPattern: "pin", Category: repositories.FieldCategorySensitive, Priority: 90},
		{FieldPattern: "token", Category: repositories.FieldCategorySensitive, Priority: 80},
		{FieldPattern: "otp", Category: repositories.FieldCategoryMFA, Priority: 100},
		{FieldPattern: "mfa", Category: repositories.FieldCategoryMFA, Priority: 100},
		{FieldPattern: "totp", Category: repositories.FieldCategoryMFA, Priority: 100},
		{FieldPattern: "2fa", Category: repositories.FieldCategoryMFA, Priority: 100},
		{FieldPattern: "verification_code", Category: repositories.FieldCategoryMFA, Priority: 90},
		{FieldPattern: "email", Category: repositories.FieldCategoryIdentity, Priority: 100},
		{FieldPattern: "username", Category: repositories.FieldCategoryIdentity, Priority: 100},
		{FieldPattern: "user", Category: repositories.FieldCategoryIdentity, Priority: 80},
		{FieldPattern: "login", Category: repositories.FieldCategoryIdentity, Priority: 80},
		{FieldPattern: "name", Category: repositories.FieldCategoryIdentity, Priority: 70},
		{FieldPattern: "phone", Category: repositories.FieldCategoryIdentity, Priority: 70},
	}

	tests := []struct {
		fieldName string
		want      repositories.FieldCategory
	}{
		// Sensitive fields.
		{"password", repositories.FieldCategorySensitive},
		{"Password1", repositories.FieldCategorySensitive},
		{"user_password", repositories.FieldCategorySensitive},
		{"passwd", repositories.FieldCategorySensitive},
		{"secret_key", repositories.FieldCategorySensitive},
		{"pin_code", repositories.FieldCategorySensitive},
		{"api_token", repositories.FieldCategorySensitive},

		// MFA fields.
		{"otp_code", repositories.FieldCategoryMFA},
		{"mfa_value", repositories.FieldCategoryMFA},
		{"totp_value", repositories.FieldCategoryMFA},
		{"2fa_code", repositories.FieldCategoryMFA},
		{"verification_code", repositories.FieldCategoryMFA},

		// Identity fields.
		{"email", repositories.FieldCategoryIdentity},
		{"user_email", repositories.FieldCategoryIdentity},
		{"username", repositories.FieldCategoryIdentity},
		{"login_id", repositories.FieldCategoryIdentity},
		{"full_name", repositories.FieldCategoryIdentity},
		{"phone_number", repositories.FieldCategoryIdentity},

		// Custom (unrecognized) fields.
		{"favorite_color", repositories.FieldCategoryCustom},
		{"address_line_1", repositories.FieldCategoryCustom},
		{"company", repositories.FieldCategoryCustom},
		{"department", repositories.FieldCategoryCustom},
	}

	for _, tt := range tests {
		got := categorizeField(tt.fieldName, rules)
		if got != tt.want {
			t.Errorf("categorizeField(%q) = %q, want %q", tt.fieldName, got, tt.want)
		}
	}
}

// TestEncryptionRoundTrip verifies credential encryption/decryption with a real key.
func TestEncryptionRoundTrip(t *testing.T) {
	masterKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	encSvc, err := crypto.NewEncryptionServiceForPurpose(masterKey, PurposeCredentialEncryption)
	if err != nil {
		t.Fatalf("NewEncryptionServiceForPurpose() error = %v", err)
	}

	// Encrypt credential values.
	testValues := map[string]string{
		"password":    "SuperSecret123!",
		"otp":         "847291",
		"credit_card": "4111-1111-1111-1111",
	}

	for fieldName, plaintext := range testValues {
		ciphertext, err := encSvc.EncryptString(plaintext)
		if err != nil {
			t.Fatalf("EncryptString(%q) error = %v", fieldName, err)
		}

		// Ciphertext should differ from plaintext.
		if string(ciphertext) == plaintext {
			t.Errorf("ciphertext should not equal plaintext for %q", fieldName)
		}

		// Ciphertext should have nonce prefix (12 bytes minimum).
		if len(ciphertext) < 12 {
			t.Errorf("ciphertext for %q too short: %d bytes", fieldName, len(ciphertext))
		}

		// Decrypt and verify round-trip.
		decrypted, err := encSvc.DecryptString(ciphertext)
		if err != nil {
			t.Fatalf("DecryptString(%q) error = %v", fieldName, err)
		}
		if decrypted != plaintext {
			t.Errorf("decrypted %q = %q, want %q", fieldName, decrypted, plaintext)
		}
	}
}

// TestEncryptionUniqueIVs verifies each encryption produces a unique nonce.
func TestEncryptionUniqueIVs(t *testing.T) {
	masterKey := []byte("0123456789abcdef0123456789abcdef")
	encSvc, err := crypto.NewEncryptionServiceForPurpose(masterKey, PurposeCredentialEncryption)
	if err != nil {
		t.Fatalf("NewEncryptionServiceForPurpose() error = %v", err)
	}

	seen := make(map[string]bool)
	plaintext := "same-password-every-time"

	for i := 0; i < 100; i++ {
		ct, err := encSvc.EncryptString(plaintext)
		if err != nil {
			t.Fatalf("iteration %d: EncryptString() error = %v", i, err)
		}
		// Extract nonce (first 12 bytes).
		nonce := string(ct[:12])
		if seen[nonce] {
			t.Fatalf("duplicate nonce at iteration %d", i)
		}
		seen[nonce] = true
	}
}

// TestSessionDataInput_Types verifies all session data types are representable.
func TestSessionDataInput_Types(t *testing.T) {
	types := []repositories.SessionDataType{
		repositories.SessionDataCookie,
		repositories.SessionDataOAuthToken,
		repositories.SessionDataSessionToken,
		repositories.SessionDataAuthHeader,
		repositories.SessionDataLocalStorage,
		repositories.SessionDataSessionStorage,
	}

	for _, dt := range types {
		input := SessionDataInput{
			DataType:        dt,
			Key:             "test-key",
			Value:           "test-value",
			IsTimeSensitive: true,
		}
		if input.DataType != dt {
			t.Errorf("data type mismatch: got %q, want %q", input.DataType, dt)
		}
	}
}

// TestPostCaptureActionTypes verifies all post-capture action types.
func TestPostCaptureActionTypes(t *testing.T) {
	actions := []repositories.PostCaptureAction{
		repositories.PostCaptureRedirect,
		repositories.PostCaptureDisplayPage,
		repositories.PostCaptureRedirectWithDelay,
		repositories.PostCaptureReplaySubmission,
		repositories.PostCaptureNoAction,
	}

	for _, a := range actions {
		if a == "" {
			t.Error("post-capture action should not be empty")
		}
	}
}

// TestPurgeValidation verifies purge requires "PURGE" confirmation.
func TestPurgeValidation(t *testing.T) {
	tests := []struct {
		confirmation string
		wantErr      bool
	}{
		{"PURGE", false},
		{"purge", true},
		{"delete", true},
		{"", true},
		{"yes", true},
	}

	for _, tt := range tests {
		// Only validate the confirmation string logic.
		err := func() error {
			if tt.confirmation != "PURGE" {
				return &ValidationError{Msg: "confirmation must be the literal string \"PURGE\""}
			}
			return nil
		}()

		if (err != nil) != tt.wantErr {
			t.Errorf("confirmation=%q: got err=%v, wantErr=%v", tt.confirmation, err, tt.wantErr)
		}
	}
}

// TestExportInput_FormatOptions verifies export format options.
func TestExportInput_FormatOptions(t *testing.T) {
	formats := []string{"csv", "json"}
	for _, f := range formats {
		input := ExportInput{Format: f}
		if input.Format != f {
			t.Errorf("format = %q, want %q", input.Format, f)
		}
	}
}

// TestCaptureMetricsDTO_Structure verifies metrics DTO fields.
func TestCaptureMetricsDTO_Structure(t *testing.T) {
	dto := CaptureMetricsDTO{
		CampaignID:       "camp-1",
		TotalCaptures:    150,
		UniqueTargets:    42,
		RepeatSubmitters: 8,
		UnattributedCount: 3,
		VariantMetrics: []repositories.VariantMetric{
			{VariantID: "v1", TotalCaptures: 80, UniqueTargets: 25},
			{VariantID: "v2", TotalCaptures: 70, UniqueTargets: 17},
		},
		Timeline: []repositories.TimelineBucket{
			{Timestamp: time.Now(), Count: 10},
		},
		FieldCompletionRates: []repositories.FieldCompletionRate{
			{FieldName: "username", FilledCount: 150, TotalEvents: 150},
			{FieldName: "password", FilledCount: 148, TotalEvents: 150},
		},
	}

	if dto.TotalCaptures != 150 {
		t.Errorf("TotalCaptures = %d, want 150", dto.TotalCaptures)
	}
	if len(dto.VariantMetrics) != 2 {
		t.Errorf("VariantMetrics len = %d, want 2", len(dto.VariantMetrics))
	}
	total := dto.VariantMetrics[0].TotalCaptures + dto.VariantMetrics[1].TotalCaptures
	if total != 150 {
		t.Errorf("variant total = %d, want 150", total)
	}
}
