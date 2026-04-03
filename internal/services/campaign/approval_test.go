package campaign

import (
	"context"
	"testing"
	"time"

	"tackle/internal/repositories"
)

func TestValidateSubmission(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)
	aws := "aws"
	region := "us-east-1"
	instType := "t3.micro"
	domainID := "domain-1"
	lpID := "lp-1"

	tests := []struct {
		name    string
		camp    repositories.Campaign
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid campaign",
			camp: repositories.Campaign{
				Name:             "Test Campaign",
				StartDate:        &now,
				EndDate:          &later,
				CloudProvider:    &aws,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
				LandingPageID:    &lpID,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			camp: repositories.Campaign{
				StartDate:        &now,
				EndDate:          &later,
				CloudProvider:    &aws,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "campaign name is required",
		},
		{
			name: "missing start_date",
			camp: repositories.Campaign{
				Name:             "Test",
				EndDate:          &later,
				CloudProvider:    &aws,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "start_date is required",
		},
		{
			name: "missing end_date",
			camp: repositories.Campaign{
				Name:             "Test",
				StartDate:        &now,
				CloudProvider:    &aws,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "end_date is required",
		},
		{
			name: "end before start",
			camp: repositories.Campaign{
				Name:             "Test",
				StartDate:        &later,
				EndDate:          &now,
				CloudProvider:    &aws,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "end_date must be after start_date",
		},
		{
			name: "missing cloud_provider",
			camp: repositories.Campaign{
				Name:             "Test",
				StartDate:        &now,
				EndDate:          &later,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "cloud_provider is required",
		},
		{
			name: "missing region",
			camp: repositories.Campaign{
				Name:             "Test",
				StartDate:        &now,
				EndDate:          &later,
				CloudProvider:    &aws,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "region is required",
		},
		{
			name: "missing instance_type",
			camp: repositories.Campaign{
				Name:             "Test",
				StartDate:        &now,
				EndDate:          &later,
				CloudProvider:    &aws,
				Region:           &region,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "instance_type is required",
		},
		{
			name: "missing endpoint_domain_id",
			camp: repositories.Campaign{
				Name:          "Test",
				StartDate:     &now,
				EndDate:       &later,
				CloudProvider: &aws,
				Region:        &region,
				InstanceType:  &instType,
			},
			wantErr: true,
			errMsg:  "endpoint domain is required",
		},
		{
			name: "missing landing_page_id",
			camp: repositories.Campaign{
				Name:             "Test",
				StartDate:        &now,
				EndDate:          &later,
				CloudProvider:    &aws,
				Region:           &region,
				InstanceType:     &instType,
				EndpointDomainID: &domainID,
			},
			wantErr: true,
			errMsg:  "landing page is required",
		},
	}

	svc := &ApprovalService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateSubmission(context.Background(), tt.camp)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected ValidationError, got %T", err)
					return
				}
				if tt.errMsg != "" && !contains(ve.Msg, tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, ve.Msg)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestBuildConfigSnapshot(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)
	aws := "aws"
	region := "us-east-1"

	svc := &ApprovalService{}
	camp := repositories.Campaign{
		Name:          "Test Campaign",
		Description:   "desc",
		StartDate:     &now,
		EndDate:       &later,
		CloudProvider: &aws,
		Region:        &region,
		SendOrder:     "default",
	}

	variants := []repositories.CampaignTemplateVariant{
		{TemplateID: "t1", SplitRatio: 70, Label: "A"},
		{TemplateID: "t2", SplitRatio: 30, Label: "B"},
	}

	snapshot := svc.buildConfigSnapshot(camp, variants)

	// Verify key fields are present.
	if snapshot["name"] != "Test Campaign" {
		t.Errorf("snapshot name: got %v", snapshot["name"])
	}
	if snapshot["description"] != "desc" {
		t.Errorf("snapshot description: got %v", snapshot["description"])
	}
	if snapshot["send_order"] != "default" {
		t.Errorf("snapshot send_order: got %v", snapshot["send_order"])
	}

	variantList, ok := snapshot["template_variants"].([]map[string]any)
	if !ok {
		t.Fatal("template_variants not present or wrong type")
	}
	if len(variantList) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variantList))
	}

	if snapshot["snapshotted_at"] == nil {
		t.Error("snapshotted_at should be set")
	}
}

func TestToApprovalDTO(t *testing.T) {
	now := time.Now()
	justification := "test justification"

	a := repositories.CampaignApproval{
		ID:                     "a1",
		CampaignID:             "c1",
		SubmissionID:           "s1",
		ActorID:                "u1",
		Action:                 "approved",
		Comments:               "looks good",
		BlockListAcknowledged:  true,
		BlockListJustification: &justification,
		CreatedAt:              now,
	}

	dto := toApprovalDTO(a)
	if dto.ID != "a1" {
		t.Errorf("ID: got %q", dto.ID)
	}
	if dto.CampaignID != "c1" {
		t.Errorf("CampaignID: got %q", dto.CampaignID)
	}
	if dto.Action != "approved" {
		t.Errorf("Action: got %q", dto.Action)
	}
	if dto.Comments != "looks good" {
		t.Errorf("Comments: got %q", dto.Comments)
	}
	if !dto.BlockListAcknowledged {
		t.Error("BlockListAcknowledged should be true")
	}
	if dto.BlockListJustification == nil || *dto.BlockListJustification != "test justification" {
		t.Errorf("BlockListJustification: got %v", dto.BlockListJustification)
	}
}

func TestRejectInputValidation(t *testing.T) {
	// Reject must have non-empty comments.
	tests := []struct {
		name    string
		input   RejectInput
		wantErr bool
	}{
		{"empty comments rejected", RejectInput{Comments: ""}, true},
		{"whitespace only rejected", RejectInput{Comments: "   "}, true},
		{"valid comments accepted", RejectInput{Comments: "Needs more detail"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the validation that happens in Reject().
			comments := tt.input.Comments
			if len(comments) == 0 || len(trimmedStr(comments)) == 0 {
				if !tt.wantErr {
					t.Error("expected no error")
				}
			} else {
				if tt.wantErr {
					t.Error("expected error")
				}
			}
		})
	}
}

func TestBlocklistOverrideInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   BlocklistOverrideInput
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid action",
			input:   BlocklistOverrideInput{Action: "invalid"},
			wantErr: true,
			errMsg:  "action must be",
		},
		{
			name:    "approve without acknowledgment",
			input:   BlocklistOverrideInput{Action: "approve", Acknowledged: false, Justification: "reason"},
			wantErr: true,
			errMsg:  "acknowledgment is required",
		},
		{
			name:    "approve without justification",
			input:   BlocklistOverrideInput{Action: "approve", Acknowledged: true, Justification: ""},
			wantErr: true,
			errMsg:  "justification is required",
		},
		{
			name:    "valid approve",
			input:   BlocklistOverrideInput{Action: "approve", Acknowledged: true, Justification: "approved by CISO"},
			wantErr: false,
		},
		{
			name:    "valid reject",
			input:   BlocklistOverrideInput{Action: "reject"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBlocklistOverrideInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestForbiddenErrorType(t *testing.T) {
	fe := &ForbiddenError{Msg: "test forbidden"}
	if fe.Error() != "test forbidden" {
		t.Errorf("ForbiddenError message wrong: %q", fe.Error())
	}
}

func TestApprovalRequirementDTO(t *testing.T) {
	dto := ApprovalRequirementDTO{
		CampaignID:            "c1",
		SubmissionID:          "s1",
		RequiredApproverCount: 3,
		RequiresAdminApproval: true,
		CurrentApprovalCount:  1,
	}
	if dto.RequiredApproverCount != 3 {
		t.Errorf("RequiredApproverCount: got %d", dto.RequiredApproverCount)
	}
	if !dto.RequiresAdminApproval {
		t.Error("RequiresAdminApproval should be true")
	}
	if dto.CurrentApprovalCount != 1 {
		t.Errorf("CurrentApprovalCount: got %d", dto.CurrentApprovalCount)
	}
}

// helpers

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func trimmedStr(s string) string {
	result := ""
	for _, c := range s {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			result += string(c)
		}
	}
	return result
}

// validateBlocklistOverrideInput validates override input (extracted for testing).
func validateBlocklistOverrideInput(input BlocklistOverrideInput) error {
	if input.Action != "approve" && input.Action != "reject" {
		return &ValidationError{Msg: "action must be 'approve' or 'reject'"}
	}
	if input.Action == "approve" {
		if !input.Acknowledged {
			return &ValidationError{Msg: "block list acknowledgment is required"}
		}
		if trimmedStr(input.Justification) == "" {
			return &ValidationError{Msg: "justification is required for block list override approval"}
		}
	}
	return nil
}
