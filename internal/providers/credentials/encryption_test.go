package credentials

import (
	"testing"
)

var testMasterKey = []byte("aaaabbbbccccddddeeeeffffgggghhhh") // 32 bytes

func newTestEncSvc(t *testing.T) *EncryptionService {
	t.Helper()
	svc, err := NewEncryptionService(testMasterKey)
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}
	return svc
}

func TestEncryptDecrypt_Namecheap(t *testing.T) {
	svc := newTestEncSvc(t)
	original := NamecheapCredentials{
		APIUser:  "ncuser",
		APIKey:   "nckey123",
		Username: "nclogin",
		ClientIP: "10.0.0.1",
	}
	ct, err := svc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var decoded NamecheapCredentials
	if err := svc.Decrypt(ct, &decoded); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decoded.APIUser != original.APIUser || decoded.APIKey != original.APIKey ||
		decoded.Username != original.Username || decoded.ClientIP != original.ClientIP {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestEncryptDecrypt_GoDaddy(t *testing.T) {
	svc := newTestEncSvc(t)
	original := GoDaddyCredentials{
		APIKey:      "gdkey",
		APISecret:   "gdsec",
		Environment: GoDaddyOTE,
	}
	ct, err := svc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var decoded GoDaddyCredentials
	if err := svc.Decrypt(ct, &decoded); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decoded.APIKey != original.APIKey || decoded.APISecret != original.APISecret || decoded.Environment != original.Environment {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestEncryptDecrypt_Route53(t *testing.T) {
	svc := newTestEncSvc(t)
	original := Route53Credentials{
		AccessKeyID:     "AKIAFAKE123",
		SecretAccessKey: "fakesecretkey",
		Region:          "us-west-2",
		IAMRoleARN:      "arn:aws:iam::123456789012:role/TestRole",
	}
	ct, err := svc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var decoded Route53Credentials
	if err := svc.Decrypt(ct, &decoded); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decoded.AccessKeyID != original.AccessKeyID || decoded.SecretAccessKey != original.SecretAccessKey ||
		decoded.Region != original.Region || decoded.IAMRoleARN != original.IAMRoleARN {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestEncryptDecrypt_AzureDNS(t *testing.T) {
	svc := newTestEncSvc(t)
	original := AzureDNSCredentials{
		TenantID:       "tenant-1234",
		ClientID:       "client-5678",
		ClientSecret:   "azuresecret",
		SubscriptionID: "sub-0000",
		ResourceGroup:  "rg-phishing",
	}
	ct, err := svc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var decoded AzureDNSCredentials
	if err := svc.Decrypt(ct, &decoded); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decoded.TenantID != original.TenantID || decoded.ClientID != original.ClientID ||
		decoded.ClientSecret != original.ClientSecret || decoded.SubscriptionID != original.SubscriptionID ||
		decoded.ResourceGroup != original.ResourceGroup {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestEncrypt_WrongKey_Fails(t *testing.T) {
	svc1 := newTestEncSvc(t)
	svc2, err := NewEncryptionService([]byte("zzzzyyyyxxxxwwwwvvvvuuuuttttssss"))
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}
	original := NamecheapCredentials{APIKey: "key"}
	ct, err := svc1.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var decoded NamecheapCredentials
	if err := svc2.Decrypt(ct, &decoded); err == nil {
		t.Error("Decrypt with wrong key should fail")
	}
}
