package credential

import (
	"testing"

	"tackle/internal/repositories"
)

func TestCategorizeField(t *testing.T) {
	rules := []repositories.FieldCategorizationRule{
		{FieldPattern: "password", Category: repositories.FieldCategorySensitive, Priority: 100},
		{FieldPattern: "email", Category: repositories.FieldCategoryIdentity, Priority: 100},
		{FieldPattern: "otp", Category: repositories.FieldCategoryMFA, Priority: 100},
		{FieldPattern: "token", Category: repositories.FieldCategorySensitive, Priority: 80},
		{FieldPattern: "username", Category: repositories.FieldCategoryIdentity, Priority: 100},
	}

	tests := []struct {
		fieldName string
		want      repositories.FieldCategory
	}{
		{"password", repositories.FieldCategorySensitive},
		{"Password", repositories.FieldCategorySensitive},
		{"user_password", repositories.FieldCategorySensitive},
		{"email", repositories.FieldCategoryIdentity},
		{"user_email", repositories.FieldCategoryIdentity},
		{"otp_code", repositories.FieldCategoryMFA},
		{"auth_token", repositories.FieldCategorySensitive},
		{"username", repositories.FieldCategoryIdentity},
		{"favorite_color", repositories.FieldCategoryCustom},
		{"address", repositories.FieldCategoryCustom},
	}

	for _, tt := range tests {
		got := categorizeField(tt.fieldName, rules)
		if got != tt.want {
			t.Errorf("categorizeField(%q) = %q, want %q", tt.fieldName, got, tt.want)
		}
	}
}

func TestStrPtr(t *testing.T) {
	if got := strPtr("hello"); got == nil || *got != "hello" {
		t.Error("strPtr should return pointer to non-empty string")
	}
	if got := strPtr(""); got != nil {
		t.Error("strPtr should return nil for empty string")
	}
}

func TestStrPtrNonEmpty(t *testing.T) {
	if got := strPtrNonEmpty("test"); got == nil || *got != "test" {
		t.Error("strPtrNonEmpty should return pointer to non-empty string")
	}
	if got := strPtrNonEmpty(""); got != nil {
		t.Error("strPtrNonEmpty should return nil for empty string")
	}
}

func TestDefaultStr(t *testing.T) {
	if got := defaultStr("value", "default"); got != "value" {
		t.Errorf("defaultStr with value should return value, got %q", got)
	}
	if got := defaultStr("", "default"); got != "default" {
		t.Errorf("defaultStr with empty should return default, got %q", got)
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Msg: "test error"}
	if err.Error() != "test error" {
		t.Errorf("ValidationError.Error() = %q, want %q", err.Error(), "test error")
	}
}

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{Msg: "not found"}
	if err.Error() != "not found" {
		t.Errorf("NotFoundError.Error() = %q, want %q", err.Error(), "not found")
	}
}

func TestForbiddenError(t *testing.T) {
	err := &ForbiddenError{Msg: "forbidden"}
	if err.Error() != "forbidden" {
		t.Errorf("ForbiddenError.Error() = %q, want %q", err.Error(), "forbidden")
	}
}

func TestCategorizeFieldEmpty(t *testing.T) {
	// No rules — everything is custom.
	got := categorizeField("anything", nil)
	if got != repositories.FieldCategoryCustom {
		t.Errorf("categorizeField with no rules should return custom, got %q", got)
	}
}

func TestCategorizeFieldCaseInsensitive(t *testing.T) {
	rules := []repositories.FieldCategorizationRule{
		{FieldPattern: "PASSWORD", Category: repositories.FieldCategorySensitive},
	}
	// Should match case-insensitively.
	got := categorizeField("my_password_field", rules)
	if got != repositories.FieldCategorySensitive {
		t.Errorf("categorizeField should be case-insensitive, got %q", got)
	}
}

func TestCaptureEventDTO_FieldsDefault(t *testing.T) {
	dto := CaptureEventDTO{}
	if dto.IsUnattributed {
		t.Error("IsUnattributed should default to false")
	}
	if dto.IsCanary {
		t.Error("IsCanary should default to false")
	}
	if dto.SubmissionSeq != 0 {
		t.Error("SubmissionSeq should default to 0")
	}
}

func TestPurposeConstant(t *testing.T) {
	if PurposeCredentialEncryption == "" {
		t.Error("PurposeCredentialEncryption should not be empty")
	}
	if PurposeCredentialEncryption != "tackle/credential-encryption" {
		t.Errorf("unexpected purpose: %s", PurposeCredentialEncryption)
	}
}
