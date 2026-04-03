package campaign

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"tackle/internal/campaign"
)

func TestBuildProgressJSON(t *testing.T) {
	msg := buildProgress{
		Type:       "campaign_build_progress",
		CampaignID: "camp-1",
		Step:       3,
		TotalSteps: totalBuildSteps,
		StepName:   stepAssignVariant,
		Status:     "completed",
	}

	if msg.Type != "campaign_build_progress" {
		t.Errorf("unexpected type: %s", msg.Type)
	}
	if msg.TotalSteps != 10 {
		t.Errorf("expected 10 total steps, got %d", msg.TotalSteps)
	}
	if msg.StepName != "Assign template variants" {
		t.Errorf("unexpected step name: %s", msg.StepName)
	}
}

func TestStepConstants(t *testing.T) {
	steps := []string{
		stepValidate, stepSnapshot, stepAssignVariant, stepCompile,
		stepStartApp, stepProvisionVM, stepDeployProxy, stepConfigureDNS,
		stepProvisionTLS, stepHealthCheck,
	}

	if len(steps) != totalBuildSteps {
		t.Errorf("expected %d step constants, got %d", totalBuildSteps, len(steps))
	}

	// Verify all step names are non-empty and unique.
	seen := make(map[string]bool)
	for _, s := range steps {
		if s == "" {
			t.Error("step name should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate step name: %s", s)
		}
		seen[s] = true
	}
}

func TestBuildStateInit(t *testing.T) {
	bs := &buildState{campaignID: "test-campaign"}
	if bs.campaignID != "test-campaign" {
		t.Errorf("unexpected campaign ID: %s", bs.campaignID)
	}
	if bs.appStarted {
		t.Error("appStarted should be false initially")
	}
	if bs.dnsCreated {
		t.Error("dnsCreated should be false initially")
	}
	if bs.tlsProvisioned {
		t.Error("tlsProvisioned should be false initially")
	}
	if bs.buildID != "" {
		t.Error("buildID should be empty initially")
	}
	if bs.endpointID != "" {
		t.Error("endpointID should be empty initially")
	}
}

func TestBuilderDepsAllFields(t *testing.T) {
	// Verify that BuilderDeps can be constructed with nil values (no panic).
	deps := BuilderDeps{}
	builder := NewCampaignBuilder(deps)
	if builder == nil {
		t.Error("NewCampaignBuilder should not return nil")
	}
}

func TestTransitionStatesExist(t *testing.T) {
	// Verify the campaign state machine supports the transitions the builder uses.
	// T5: building → ready (system)
	_, err := campaign.ValidateTransition(campaign.StateBuilding, campaign.StateReady, "system")
	if err != nil {
		t.Errorf("T5 (building→ready) should be valid for system: %v", err)
	}

	// T6: building → draft (system)
	_, err = campaign.ValidateTransition(campaign.StateBuilding, campaign.StateDraft, "system")
	if err != nil {
		t.Errorf("T6 (building→draft) should be valid for system: %v", err)
	}

	// T4: approved → building (operator)
	_, err = campaign.ValidateTransition(campaign.StateApproved, campaign.StateBuilding, "operator")
	if err != nil {
		t.Errorf("T4 (approved→building) should be valid for operator: %v", err)
	}
}

func TestDuplicateBuildRejection(t *testing.T) {
	// Verify that the state machine rejects building → building.
	_, err := campaign.ValidateTransition(campaign.StateBuilding, campaign.StateBuilding, "operator")
	if err == nil {
		t.Error("building→building should be rejected")
	}

	_, err = campaign.ValidateTransition(campaign.StateBuilding, campaign.StateBuilding, "system")
	if err == nil {
		t.Error("building→building should be rejected for system too")
	}
}

func TestDeterministicToken(t *testing.T) {
	secret := []byte("test-tracking-secret-32-bytes-ok")
	builder := NewCampaignBuilder(BuilderDeps{TrackingSecret: secret})

	campaignID := "campaign-abc-123"
	targetID := "target-xyz-789"

	// Same inputs produce the same token.
	token1 := builder.deterministicToken(campaignID, targetID)
	token2 := builder.deterministicToken(campaignID, targetID)
	if token1 != token2 {
		t.Errorf("expected deterministic tokens to match: %q != %q", token1, token2)
	}

	// Token is 16 characters.
	if len(token1) != 16 {
		t.Errorf("expected token length 16, got %d: %q", len(token1), token1)
	}

	// Different targets produce different tokens.
	token3 := builder.deterministicToken(campaignID, "target-other")
	if token1 == token3 {
		t.Error("different targets should produce different tokens")
	}

	// Different campaigns produce different tokens.
	token4 := builder.deterministicToken("campaign-other", targetID)
	if token1 == token4 {
		t.Error("different campaigns should produce different tokens")
	}

	// Token is valid base64url (first 16 chars of full encoding).
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(campaignID))
	mac.Write([]byte{0x00})
	mac.Write([]byte(targetID))
	digest := mac.Sum(nil)
	expected := base64.RawURLEncoding.EncodeToString(digest)[:16]
	if token1 != expected {
		t.Errorf("token mismatch: got %q, expected %q", token1, expected)
	}
}

func TestDeterministicTokenURLSafe(t *testing.T) {
	secret := []byte("another-secret-for-url-safety!!")
	builder := NewCampaignBuilder(BuilderDeps{TrackingSecret: secret})

	// Generate tokens for many campaign+target pairs and verify all are URL-safe.
	for i := 0; i < 100; i++ {
		token := builder.deterministicToken(
			"campaign-"+string(rune('A'+i%26)),
			"target-"+string(rune('a'+i%26)),
		)
		for _, ch := range token {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
				t.Errorf("token contains non-URL-safe character %q in %q", ch, token)
			}
		}
	}
}

// --- ECOMP-06: State Machine Gate Tests ---

type mockSMTPValidator struct {
	results []SMTPValidationResult
	err     error
}

func (m *mockSMTPValidator) ValidateCampaignProfiles(_ context.Context, _ string) ([]SMTPValidationResult, error) {
	return m.results, m.err
}

type mockEmailAuthValidator struct {
	result EmailAuthResult
	err    error
}

func (m *mockEmailAuthValidator) ValidateEmailAuth(_ context.Context, _, _, _, _, _ string) (EmailAuthResult, error) {
	return m.result, m.err
}

func TestSMTPValidationResult_Fields(t *testing.T) {
	// All pass.
	results := []SMTPValidationResult{
		{Success: true, ErrorDetail: ""},
		{Success: true, ErrorDetail: ""},
	}
	failCount := 0
	for _, r := range results {
		if !r.Success {
			failCount++
		}
	}
	if failCount != 0 {
		t.Errorf("expected 0 failures, got %d", failCount)
	}

	// One fail.
	results[1] = SMTPValidationResult{Success: false, ErrorDetail: "connection refused"}
	failCount = 0
	for _, r := range results {
		if !r.Success {
			failCount++
		}
	}
	if failCount != 1 {
		t.Errorf("expected 1 failure, got %d", failCount)
	}
}

func TestEmailAuthResult_Fields(t *testing.T) {
	result := EmailAuthResult{
		SPFStatus:   "pass",
		DKIMStatus:  "pass",
		DMARCStatus: "pass",
	}
	if result.SPFStatus != "pass" {
		t.Error("SPF should be pass")
	}
	if result.DKIMStatus != "pass" {
		t.Error("DKIM should be pass")
	}
	if result.DMARCStatus != "pass" {
		t.Error("DMARC should be pass")
	}

	// Failed case.
	result.SPFStatus = "fail"
	if result.SPFStatus != "fail" {
		t.Error("SPF should be fail")
	}
}

func TestBuilderDepsWithValidators(t *testing.T) {
	// Verify deps can hold validators.
	deps := BuilderDeps{
		SMTPValidator: &mockSMTPValidator{
			results: []SMTPValidationResult{{Success: true}},
		},
		EmailAuthValidator: &mockEmailAuthValidator{
			result: EmailAuthResult{SPFStatus: "pass", DKIMStatus: "pass", DMARCStatus: "pass"},
		},
	}
	builder := NewCampaignBuilder(deps)
	if builder == nil {
		t.Error("NewCampaignBuilder should not return nil with validators")
	}
	if builder.deps.SMTPValidator == nil {
		t.Error("SMTPValidator should be set")
	}
	if builder.deps.EmailAuthValidator == nil {
		t.Error("EmailAuthValidator should be set")
	}
}

func TestEmailDeliveryHookTransitions(t *testing.T) {
	// Verify the state machine supports all transitions the hooks rely on.
	// Ready → Active (launch)
	_, err := campaign.ValidateTransition(campaign.StateReady, campaign.StateActive, "operator")
	if err != nil {
		t.Errorf("ready→active should be valid: %v", err)
	}

	// Active → Paused
	_, err = campaign.ValidateTransition(campaign.StateActive, campaign.StatePaused, "operator")
	if err != nil {
		t.Errorf("active→paused should be valid: %v", err)
	}

	// Paused → Active (resume)
	_, err = campaign.ValidateTransition(campaign.StatePaused, campaign.StateActive, "operator")
	if err != nil {
		t.Errorf("paused→active should be valid: %v", err)
	}

	// Active → Completed
	_, err = campaign.ValidateTransition(campaign.StateActive, campaign.StateCompleted, "operator")
	if err != nil {
		t.Errorf("active→completed should be valid: %v", err)
	}
}
