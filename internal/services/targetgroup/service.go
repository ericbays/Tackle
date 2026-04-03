// Package targetgroup implements business logic for target group management.
package targetgroup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// ValidationError is returned when input fails validation.
type ValidationError struct{ Msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Msg }

// ConflictError is returned when a uniqueness constraint would be violated.
type ConflictError struct{ Msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.Msg }

// NotFoundError is returned when a resource is not found.
type NotFoundError struct{ Msg string }

// Error implements the error interface.
func (e *NotFoundError) Error() string { return e.Msg }

// GroupDTO is the API-safe representation of a target group.
type GroupDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// GroupDetailDTO includes member list.
type GroupDetailDTO struct {
	GroupDTO
	Members    []MemberDTO `json:"members"`
	MemberPage int         `json:"member_page"`
	MemberTotal int        `json:"member_total"`
}

// MemberDTO is the API-safe representation of a group member.
type MemberDTO struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	FirstName  *string `json:"first_name,omitempty"`
	LastName   *string `json:"last_name,omitempty"`
	Department *string `json:"department,omitempty"`
	Title      *string `json:"title,omitempty"`
}

// CreateInput holds fields for creating a group.
type CreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateInput holds fields for updating a group.
type UpdateInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// MembershipInput holds target IDs for add/remove operations.
type MembershipInput struct {
	TargetIDs []string `json:"target_ids"`
}

// ResolvedTargetDTO is the API-safe representation of a resolved campaign target.
type ResolvedTargetDTO struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	FirstName  *string  `json:"first_name,omitempty"`
	LastName   *string  `json:"last_name,omitempty"`
	Department *string  `json:"department,omitempty"`
	Title      *string  `json:"title,omitempty"`
	Sources    []string `json:"sources"`
}

// ResolutionResultDTO is the result of resolving targets for a campaign.
type ResolutionResultDTO struct {
	Targets           []ResolvedTargetDTO `json:"targets"`
	TotalTargets      int                 `json:"total_targets"`
	DuplicatesRemoved int                 `json:"duplicates_removed"`
}

// Service implements target group business logic.
type Service struct {
	repo     *repositories.TargetGroupRepository
	auditSvc *auditsvc.AuditService
}

// NewService creates a new target group Service.
func NewService(repo *repositories.TargetGroupRepository, auditSvc *auditsvc.AuditService) *Service {
	return &Service{repo: repo, auditSvc: auditSvc}
}

// Create validates and persists a new group.
func (s *Service) Create(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (GroupDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return GroupDTO{}, &ValidationError{Msg: "name is required"}
	}
	if len(name) > 255 {
		return GroupDTO{}, &ValidationError{Msg: "name must be 255 characters or fewer"}
	}
	desc := strings.TrimSpace(input.Description)
	if len(desc) > 1024 {
		return GroupDTO{}, &ValidationError{Msg: "description must be 1024 characters or fewer"}
	}

	// Check uniqueness.
	_, err := s.repo.GetByName(ctx, name)
	if err == nil {
		return GroupDTO{}, &ConflictError{Msg: "a group with this name already exists"}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return GroupDTO{}, fmt.Errorf("check group name: %w", err)
	}

	g, err := s.repo.Create(ctx, repositories.TargetGroup{
		Name:        name,
		Description: desc,
		CreatedBy:   actorID,
	})
	if err != nil {
		return GroupDTO{}, fmt.Errorf("create group: %w", err)
	}

	resourceType := "target_group"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target_group.create",
		ResourceType:  &resourceType,
		ResourceID:    &g.ID,
		Details:       map[string]any{"name": name},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return groupToDTO(g, 0), nil
}

// Get retrieves a group by ID with paginated members.
func (s *Service) Get(ctx context.Context, id string, memberPage, memberPerPage int) (GroupDetailDTO, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GroupDetailDTO{}, &NotFoundError{Msg: "group not found"}
		}
		return GroupDetailDTO{}, fmt.Errorf("get group: %w", err)
	}

	count, err := s.repo.MemberCount(ctx, id)
	if err != nil {
		return GroupDetailDTO{}, fmt.Errorf("group member count: %w", err)
	}

	members, memberTotal, err := s.repo.ListMembers(ctx, id, memberPage, memberPerPage)
	if err != nil {
		return GroupDetailDTO{}, fmt.Errorf("list group members: %w", err)
	}

	dto := GroupDetailDTO{
		GroupDTO:    groupToDTO(g, count),
		Members:    targetsToMemberDTOs(members),
		MemberPage: memberPage,
		MemberTotal: memberTotal,
	}
	return dto, nil
}

// List returns groups with member counts.
func (s *Service) List(ctx context.Context, f repositories.TargetGroupFilters) ([]GroupDTO, int, error) {
	groups, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("list groups: %w", err)
	}

	dtos := make([]GroupDTO, 0, len(groups))
	for _, g := range groups {
		dtos = append(dtos, groupToDTO(g.TargetGroup, g.MemberCount))
	}
	return dtos, total, nil
}

// Update modifies a group's name or description.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (GroupDTO, error) {
	// Validate name if provided.
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return GroupDTO{}, &ValidationError{Msg: "name cannot be empty"}
		}
		if len(name) > 255 {
			return GroupDTO{}, &ValidationError{Msg: "name must be 255 characters or fewer"}
		}
		input.Name = &name

		// Check uniqueness against other groups.
		existing, err := s.repo.GetByName(ctx, name)
		if err == nil && existing.ID != id {
			return GroupDTO{}, &ConflictError{Msg: "a group with this name already exists"}
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return GroupDTO{}, fmt.Errorf("check group name: %w", err)
		}
	}
	if input.Description != nil {
		desc := strings.TrimSpace(*input.Description)
		if len(desc) > 1024 {
			return GroupDTO{}, &ValidationError{Msg: "description must be 1024 characters or fewer"}
		}
		input.Description = &desc
	}

	g, err := s.repo.Update(ctx, id, input.Name, input.Description)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GroupDTO{}, &NotFoundError{Msg: "group not found"}
		}
		return GroupDTO{}, fmt.Errorf("update group: %w", err)
	}

	count, err := s.repo.MemberCount(ctx, id)
	if err != nil {
		return GroupDTO{}, fmt.Errorf("group member count: %w", err)
	}

	resourceType := "target_group"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target_group.update",
		ResourceType:  &resourceType,
		ResourceID:    &g.ID,
		Details:       map[string]any{"name": g.Name},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return groupToDTO(g, count), nil
}

// Delete removes a group. Does NOT delete member targets.
func (s *Service) Delete(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Msg: "group not found"}
		}
		return fmt.Errorf("get group: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	resourceType := "target_group"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target_group.delete",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		Details:       map[string]any{"name": g.Name},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return nil
}

// AddMembers adds targets to a group. Idempotent.
func (s *Service) AddMembers(ctx context.Context, groupID string, input MembershipInput, actorID, actorName, ip, correlationID string) (int, error) {
	if len(input.TargetIDs) == 0 {
		return 0, &ValidationError{Msg: "target_ids is required"}
	}

	// Verify group exists.
	if _, err := s.repo.GetByID(ctx, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, &NotFoundError{Msg: "group not found"}
		}
		return 0, fmt.Errorf("get group: %w", err)
	}

	added, err := s.repo.AddMembers(ctx, groupID, input.TargetIDs, actorID)
	if err != nil {
		return 0, fmt.Errorf("add members: %w", err)
	}

	resourceType := "target_group"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target_group.members.add",
		ResourceType:  &resourceType,
		ResourceID:    &groupID,
		Details:       map[string]any{"target_count": len(input.TargetIDs), "added": added},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return added, nil
}

// RemoveMembers removes targets from a group. Idempotent.
func (s *Service) RemoveMembers(ctx context.Context, groupID string, input MembershipInput, actorID, actorName, ip, correlationID string) (int, error) {
	if len(input.TargetIDs) == 0 {
		return 0, &ValidationError{Msg: "target_ids is required"}
	}

	if _, err := s.repo.GetByID(ctx, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, &NotFoundError{Msg: "group not found"}
		}
		return 0, fmt.Errorf("get group: %w", err)
	}

	removed, err := s.repo.RemoveMembers(ctx, groupID, input.TargetIDs)
	if err != nil {
		return 0, fmt.Errorf("remove members: %w", err)
	}

	resourceType := "target_group"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target_group.members.remove",
		ResourceType:  &resourceType,
		ResourceID:    &groupID,
		Details:       map[string]any{"target_count": len(input.TargetIDs), "removed": removed},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return removed, nil
}

// ResolveTargets computes the deduplicated target list for a campaign.
func (s *Service) ResolveTargets(ctx context.Context, campaignID string) (ResolutionResultDTO, error) {
	resolved, err := s.repo.ResolveTargetsForCampaign(ctx, campaignID)
	if err != nil {
		return ResolutionResultDTO{}, fmt.Errorf("resolve targets: %w", err)
	}

	dtos := make([]ResolvedTargetDTO, 0, len(resolved))
	for _, rt := range resolved {
		dtos = append(dtos, ResolvedTargetDTO{
			ID:         rt.ID,
			Email:      rt.Email,
			FirstName:  rt.FirstName,
			LastName:   rt.LastName,
			Department: rt.Department,
			Title:      rt.Title,
			Sources:    rt.Sources,
		})
	}

	// Duplicates removed is calculated from total sources minus unique targets.
	totalSources := 0
	for _, rt := range resolved {
		totalSources += len(rt.Sources)
	}
	dupsRemoved := totalSources - len(resolved)

	return ResolutionResultDTO{
		Targets:           dtos,
		TotalTargets:      len(dtos),
		DuplicatesRemoved: dupsRemoved,
	}, nil
}

// AssignGroupToCampaign assigns a group to a campaign.
func (s *Service) AssignGroupToCampaign(ctx context.Context, campaignID, groupID, actorID, actorName, ip, correlationID string) error {
	if _, err := s.repo.GetByID(ctx, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Msg: "group not found"}
		}
		return fmt.Errorf("get group: %w", err)
	}

	if err := s.repo.AssignGroupToCampaign(ctx, campaignID, groupID, actorID); err != nil {
		return fmt.Errorf("assign group to campaign: %w", err)
	}

	resourceType := "campaign"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "campaign.group.assign",
		ResourceType:  &resourceType,
		ResourceID:    &campaignID,
		Details:       map[string]any{"group_id": groupID},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return nil
}

// UnassignGroupFromCampaign removes a group assignment from a campaign.
func (s *Service) UnassignGroupFromCampaign(ctx context.Context, campaignID, groupID, actorID, actorName, ip, correlationID string) error {
	if err := s.repo.UnassignGroupFromCampaign(ctx, campaignID, groupID); err != nil {
		return fmt.Errorf("unassign group from campaign: %w", err)
	}

	resourceType := "campaign"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "campaign.group.unassign",
		ResourceType:  &resourceType,
		ResourceID:    &campaignID,
		Details:       map[string]any{"group_id": groupID},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return nil
}

// ListCampaignGroups returns groups assigned to a campaign.
func (s *Service) ListCampaignGroups(ctx context.Context, campaignID string) ([]GroupDTO, error) {
	groups, err := s.repo.ListCampaignGroups(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list campaign groups: %w", err)
	}

	dtos := make([]GroupDTO, 0, len(groups))
	for _, g := range groups {
		dtos = append(dtos, groupToDTO(g.TargetGroup, g.MemberCount))
	}
	return dtos, nil
}

func groupToDTO(g repositories.TargetGroup, memberCount int) GroupDTO {
	return GroupDTO{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		MemberCount: memberCount,
		CreatedBy:   g.CreatedBy,
		CreatedAt:   g.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   g.UpdatedAt.Format(time.RFC3339),
	}
}

func targetsToMemberDTOs(targets []repositories.Target) []MemberDTO {
	dtos := make([]MemberDTO, 0, len(targets))
	for _, t := range targets {
		dtos = append(dtos, MemberDTO{
			ID:         t.ID,
			Email:      t.Email,
			FirstName:  t.FirstName,
			LastName:   t.LastName,
			Department: t.Department,
			Title:      t.Title,
		})
	}
	return dtos
}
