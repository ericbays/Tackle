package campaign

import (
	"testing"
	"time"

	"tackle/internal/campaign"
	"tackle/internal/repositories"
)

func TestValidateSendOrder(t *testing.T) {
	valid := []string{"default", "alphabetical", "department", "custom", "randomized"}
	for _, v := range valid {
		if err := validateSendOrder(v); err != nil {
			t.Errorf("validateSendOrder(%q) should be valid, got: %v", v, err)
		}
	}

	invalid := []string{"", "reverse", "random"}
	for _, v := range invalid {
		if err := validateSendOrder(v); err == nil {
			t.Errorf("validateSendOrder(%q) should be invalid", v)
		}
	}
}

func TestDeterministicHash(t *testing.T) {
	h1 := deterministicHash("campaign-1", "target-1")
	h2 := deterministicHash("campaign-1", "target-1")
	if h1 != h2 {
		t.Error("hash should be deterministic")
	}

	h3 := deterministicHash("campaign-1", "target-2")
	if h1 == h3 {
		t.Error("different inputs should produce different hash")
	}

	h4 := deterministicHash("campaign-2", "target-1")
	if h1 == h4 {
		t.Error("different campaign should produce different hash")
	}
}

func TestToCampaignDTO(t *testing.T) {
	c := repositories.Campaign{
		ID:               "test-id",
		Name:             "Test Campaign",
		Description:      "A test",
		CurrentState:     "draft",
		StateChangedAt:   time.Now(),
		SendOrder:        "default",
		GracePeriodHours: 72,
		Configuration:    map[string]any{"key": "val"},
		CreatedBy:        "user-1",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	dto := toCampaignDTO(c)
	if dto.ID != "test-id" {
		t.Errorf("ID mismatch: got %q", dto.ID)
	}
	if dto.Name != "Test Campaign" {
		t.Errorf("Name mismatch: got %q", dto.Name)
	}
	if dto.CurrentState != "draft" {
		t.Errorf("State mismatch: got %q", dto.CurrentState)
	}
	if dto.GracePeriodHours != 72 {
		t.Errorf("GracePeriodHours mismatch: got %d", dto.GracePeriodHours)
	}
}

func TestToVariantDTO(t *testing.T) {
	v := repositories.CampaignTemplateVariant{
		ID: "v1", CampaignID: "c1", TemplateID: "t1",
		SplitRatio: 70, Label: "Variant A", CreatedAt: time.Now(),
	}
	dto := toVariantDTO(v)
	if dto.SplitRatio != 70 {
		t.Errorf("SplitRatio mismatch: got %d", dto.SplitRatio)
	}
	if dto.Label != "Variant A" {
		t.Errorf("Label mismatch: got %q", dto.Label)
	}
}

func TestVariantRatioValidation(t *testing.T) {
	tests := []struct {
		name    string
		ratios  []int
		wantErr bool
	}{
		{"valid 100", []int{100}, false},
		{"valid 50/50", []int{50, 50}, false},
		{"valid 70/30", []int{70, 30}, false},
		{"invalid 80", []int{80}, true},
		{"invalid 60/60", []int{60, 60}, true},
		{"invalid 0", []int{0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := 0
			hasInvalid := false
			for _, r := range tt.ratios {
				if r < 1 || r > 100 {
					hasInvalid = true
				}
				total += r
			}
			isErr := hasInvalid || total != 100
			if isErr != tt.wantErr {
				t.Errorf("ratios %v: expected error=%v, got error=%v", tt.ratios, tt.wantErr, isErr)
			}
		})
	}
}

func TestValidateSplitRatios(t *testing.T) {
	tests := []struct {
		name    string
		ratios  []int
		wantErr bool
	}{
		{"valid 100", []int{100}, false},
		{"valid 50/50", []int{50, 50}, false},
		{"valid 60/20/20", []int{60, 20, 20}, false},
		{"invalid sum 80", []int{80}, true},
		{"invalid sum 120", []int{60, 60}, true},
		{"invalid negative", []int{-10, 110}, true},
		{"empty", []int{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSplitRatios(tt.ratios)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSplitRatios(%v) error = %v, wantErr %v", tt.ratios, err, tt.wantErr)
			}
		})
	}
}

func TestDeterministicAssignment_Distribution(t *testing.T) {
	// Simulate the deterministic assignment algorithm for 1000 targets at 50/50.
	totalTargets := 1000
	ratio1 := 50 // variant A

	// Expected count per variant.
	countA := totalTargets * ratio1 / 100
	countB := totalTargets - countA // remainder to variant B

	if countA < 450 || countA > 550 {
		t.Errorf("variant A assignment = %d, expected ~500", countA)
	}
	if countB < 450 || countB > 550 {
		t.Errorf("variant B assignment = %d, expected ~500", countB)
	}
	if countA+countB != totalTargets {
		t.Errorf("total assigned = %d, expected %d", countA+countB, totalTargets)
	}
}

func TestDeterministicAssignment_ThreeVariants(t *testing.T) {
	totalTargets := 1000
	ratios := []int{60, 20, 20}
	assigned := 0
	counts := make([]int, len(ratios))

	for vi, ratio := range ratios {
		var count int
		if vi == len(ratios)-1 {
			count = totalTargets - assigned
		} else {
			count = totalTargets * ratio / 100
			if count == 0 && ratio > 0 {
				count = 1
			}
		}
		counts[vi] = count
		assigned += count
	}

	if assigned != totalTargets {
		t.Errorf("total assigned = %d, expected %d", assigned, totalTargets)
	}
	if counts[0] != 600 {
		t.Errorf("variant 0 (60%%) = %d, expected 600", counts[0])
	}
	if counts[1] != 200 {
		t.Errorf("variant 1 (20%%) = %d, expected 200", counts[1])
	}
	if counts[2] != 200 {
		t.Errorf("variant 2 (20%%) = %d, expected 200", counts[2])
	}
}

func TestStateMutableCheck(t *testing.T) {
	if !campaign.StateDraft.IsMutable() {
		t.Error("draft should be mutable")
	}
	if campaign.StateActive.IsMutable() {
		t.Error("active should not be mutable")
	}
	if campaign.StateApproved.IsMutable() {
		t.Error("approved should not be mutable")
	}
}

func TestErrorTypes(t *testing.T) {
	ve := &ValidationError{Msg: "test validation"}
	if ve.Error() != "test validation" {
		t.Errorf("ValidationError message wrong: %q", ve.Error())
	}

	ce := &ConflictError{Msg: "test conflict"}
	if ce.Error() != "test conflict" {
		t.Errorf("ConflictError message wrong: %q", ce.Error())
	}

	ne := &NotFoundError{Msg: "test not found"}
	if ne.Error() != "test not found" {
		t.Errorf("NotFoundError message wrong: %q", ne.Error())
	}
}
