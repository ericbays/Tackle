package targetgroups

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/repositories"
	blocklistsvc "tackle/internal/services/blocklist"
	targetgroupsvc "tackle/internal/services/targetgroup"
	"tackle/pkg/response"
)

// --- Target Group Handlers ---

// ListGroups handles GET /api/v1/target-groups.
func (d *Deps) ListGroups(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	filters := repositories.TargetGroupFilters{
		Name: q.Get("name"),
	}
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filters.Page = v
		}
	}
	if p := q.Get("per_page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filters.PerPage = v
		}
	}

	dtos, total, err := d.GroupSvc.List(r.Context(), filters)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list groups", http.StatusInternalServerError, correlationID)
		return
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	perPage := filters.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	totalPages := (total + perPage - 1) / perPage

	response.List(w, dtos, response.Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// CreateGroup handles POST /api/v1/target-groups.
func (d *Deps) CreateGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input targetgroupsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.GroupSvc.Create(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// GetGroup handles GET /api/v1/target-groups/{id}.
func (d *Deps) GetGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	memberPage := 1
	memberPerPage := 25
	if p := r.URL.Query().Get("member_page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			memberPage = v
		}
	}
	if p := r.URL.Query().Get("member_per_page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			memberPerPage = v
		}
	}

	dto, err := d.GroupSvc.Get(r.Context(), id, memberPage, memberPerPage)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateGroup handles PUT /api/v1/target-groups/{id}.
func (d *Deps) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input targetgroupsvc.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.GroupSvc.Update(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteGroup handles DELETE /api/v1/target-groups/{id}.
func (d *Deps) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.GroupSvc.Delete(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddMembers handles POST /api/v1/target-groups/{id}/members.
func (d *Deps) AddMembers(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input targetgroupsvc.MembershipInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	added, err := d.GroupSvc.AddMembers(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]int{"added": added})
}

// RemoveMembers handles DELETE /api/v1/target-groups/{id}/members.
func (d *Deps) RemoveMembers(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input targetgroupsvc.MembershipInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	removed, err := d.GroupSvc.RemoveMembers(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]int{"removed": removed})
}

// ResolveTargets handles GET /api/v1/campaigns/{id}/resolve-targets.
func (d *Deps) ResolveTargets(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	result, err := d.GroupSvc.ResolveTargets(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to resolve targets", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// AssignGroup handles POST /api/v1/campaigns/{id}/target-groups.
func (d *Deps) AssignGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	var input struct {
		GroupID string `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.GroupSvc.AssignGroupToCampaign(r.Context(), campaignID, input.GroupID, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]string{"status": "assigned"})
}

// UnassignGroup handles DELETE /api/v1/campaigns/{id}/target-groups/{groupId}.
func (d *Deps) UnassignGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")
	groupID := chi.URLParam(r, "groupId")

	if err := d.GroupSvc.UnassignGroupFromCampaign(r.Context(), campaignID, groupID, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListCampaignGroups handles GET /api/v1/campaigns/{id}/target-groups.
func (d *Deps) ListCampaignGroups(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	groups, err := d.GroupSvc.ListCampaignGroups(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list campaign groups", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, groups)
}

// --- Block List Handlers ---

// ListBlocklist handles GET /api/v1/blocklist.
func (d *Deps) ListBlocklist(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	filters := repositories.BlocklistFilters{
		Pattern: q.Get("pattern"),
		Reason:  q.Get("reason"),
	}
	if active := q.Get("is_active"); active != "" {
		b := active == "true"
		filters.IsActive = &b
	}
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filters.Page = v
		}
	}
	if p := q.Get("per_page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filters.PerPage = v
		}
	}

	dtos, total, err := d.BlocklistSvc.List(r.Context(), filters)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list blocklist", http.StatusInternalServerError, correlationID)
		return
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	perPage := filters.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	totalPages := (total + perPage - 1) / perPage

	response.List(w, dtos, response.Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// CreateBlocklistEntry handles POST /api/v1/blocklist.
func (d *Deps) CreateBlocklistEntry(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input blocklistsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.BlocklistSvc.Create(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// GetBlocklistEntry handles GET /api/v1/blocklist/{id}.
func (d *Deps) GetBlocklistEntry(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.BlocklistSvc.Get(r.Context(), id)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeactivateBlocklistEntry handles PUT /api/v1/blocklist/{id}/deactivate.
func (d *Deps) DeactivateBlocklistEntry(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	dto, err := d.BlocklistSvc.Deactivate(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// ReactivateBlocklistEntry handles PUT /api/v1/blocklist/{id}/reactivate.
func (d *Deps) ReactivateBlocklistEntry(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	dto, err := d.BlocklistSvc.Reactivate(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// CheckBlocklist handles GET /api/v1/blocklist/check.
func (d *Deps) CheckBlocklist(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	email := r.URL.Query().Get("email")
	if email == "" {
		response.Error(w, "BAD_REQUEST", "email query parameter is required", http.StatusBadRequest, correlationID)
		return
	}

	result, err := d.BlocklistSvc.CheckEmail(r.Context(), email)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// GetBlocklistReview handles GET /api/v1/campaigns/{id}/blocklist-review.
func (d *Deps) GetBlocklistReview(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	override, err := d.BlocklistSvc.GetOverride(r.Context(), campaignID)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, override)
}

// BlocklistOverride handles POST /api/v1/campaigns/{id}/blocklist-override.
func (d *Deps) BlocklistOverride(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input blocklistsvc.OverrideInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	campaignID := chi.URLParam(r, "id")

	// Get the current override for this campaign.
	existing, err := d.BlocklistSvc.GetOverride(r.Context(), campaignID)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}

	var result blocklistsvc.OverrideDTO
	switch input.Action {
	case "approve":
		result, err = d.BlocklistSvc.ApproveOverride(r.Context(), existing.ID, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	case "reject":
		result, err = d.BlocklistSvc.RejectOverride(r.Context(), existing.ID, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	default:
		response.Error(w, "BAD_REQUEST", "action must be 'approve' or 'reject'", http.StatusBadRequest, correlationID)
		return
	}
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// BulkAddToGroup handles POST /api/v1/targets/bulk/add-to-group.
func (d *Deps) BulkAddToGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input struct {
		TargetIDs []string `json:"target_ids"`
		GroupID   string   `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if len(input.TargetIDs) == 0 || input.GroupID == "" {
		response.Error(w, "BAD_REQUEST", "target_ids and group_id are required", http.StatusBadRequest, correlationID)
		return
	}

	count, err := d.GroupSvc.AddMembers(r.Context(), input.GroupID, targetgroupsvc.MembershipInput{TargetIDs: input.TargetIDs}, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]int{"added_count": count})
}

// BulkRemoveFromGroup handles POST /api/v1/targets/bulk/remove-from-group.
func (d *Deps) BulkRemoveFromGroup(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input struct {
		TargetIDs []string `json:"target_ids"`
		GroupID   string   `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if len(input.TargetIDs) == 0 || input.GroupID == "" {
		response.Error(w, "BAD_REQUEST", "target_ids and group_id are required", http.StatusBadRequest, correlationID)
		return
	}

	count, err := d.GroupSvc.RemoveMembers(r.Context(), input.GroupID, targetgroupsvc.MembershipInput{TargetIDs: input.TargetIDs}, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeGroupError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]int{"removed_count": count})
}

// ListOverrides handles GET /api/v1/blocklist-overrides.
func (d *Deps) ListOverrides(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	pendingOnly := r.URL.Query().Get("status") == "pending"

	overrides, err := d.BlocklistSvc.ListOverrides(r.Context(), pendingOnly)
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, overrides)
}

// DecideOverride handles POST /api/v1/blocklist-overrides/{id}/decide.
func (d *Deps) DecideOverride(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	overrideID := chi.URLParam(r, "id")

	var input blocklistsvc.OverrideInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	var result blocklistsvc.OverrideDTO
	var err error
	switch input.Action {
	case "approve":
		result, err = d.BlocklistSvc.ApproveOverride(r.Context(), overrideID, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	case "reject":
		result, err = d.BlocklistSvc.RejectOverride(r.Context(), overrideID, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	default:
		response.Error(w, "BAD_REQUEST", "action must be 'approve' or 'reject'", http.StatusBadRequest, correlationID)
		return
	}
	if err != nil {
		writeBlocklistError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// --- Canary Target Handlers ---

// DesignateCanary handles POST /api/v1/campaigns/{id}/canary-targets.
func (d *Deps) DesignateCanary(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	var input struct {
		TargetIDs []string `json:"target_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if len(input.TargetIDs) == 0 {
		response.Error(w, "BAD_REQUEST", "target_ids is required", http.StatusBadRequest, correlationID)
		return
	}

	added, err := d.CanaryRepo.Designate(r.Context(), campaignID, input.TargetIDs, claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to designate canary targets", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, map[string]int{"designated": added})
}

// UndesignateCanary handles DELETE /api/v1/campaigns/{id}/canary-targets.
func (d *Deps) UndesignateCanary(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	var input struct {
		TargetIDs []string `json:"target_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	removed, err := d.CanaryRepo.Undesignate(r.Context(), campaignID, input.TargetIDs)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to undesignate canary targets", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, map[string]int{"undesignated": removed})
}

// ListCanary handles GET /api/v1/campaigns/{id}/canary-targets.
func (d *Deps) ListCanary(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	targets, err := d.CanaryRepo.List(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list canary targets", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, targets)
}

// --- helpers ---

func writeGroupError(w http.ResponseWriter, err error, correlationID string) {
	var ve *targetgroupsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *targetgroupsvc.ConflictError
	if errors.As(err, &ce) {
		response.Error(w, "CONFLICT", ce.Error(), http.StatusConflict, correlationID)
		return
	}
	var nf *targetgroupsvc.NotFoundError
	if errors.As(err, &nf) {
		response.Error(w, "NOT_FOUND", nf.Error(), http.StatusNotFound, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

func writeBlocklistError(w http.ResponseWriter, err error, correlationID string) {
	var ve *blocklistsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *blocklistsvc.ConflictError
	if errors.As(err, &ce) {
		response.Error(w, "CONFLICT", ce.Error(), http.StatusConflict, correlationID)
		return
	}
	var nf *blocklistsvc.NotFoundError
	if errors.As(err, &nf) {
		response.Error(w, "NOT_FOUND", nf.Error(), http.StatusNotFound, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}
