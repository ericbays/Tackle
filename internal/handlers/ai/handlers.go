// Package ai provides HTTP handlers for AI integration endpoints.
// All endpoints currently return 501 Not Implemented as AI integration is deferred.
package ai

import (
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// Deps holds shared dependencies for AI handlers.
type Deps struct{}

// ListProposals handles GET /api/v1/ai/proposals.
func (d *Deps) ListProposals(w http.ResponseWriter, r *http.Request) {
	response.Success(w, []any{})
}

// GetProposal handles GET /api/v1/ai/proposals/{id}.
func (d *Deps) GetProposal(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	response.Error(w, "NOT_FOUND", "AI proposals not available", http.StatusNotFound, correlationID)
}

// ReviewProposal handles POST /api/v1/ai/proposals/{id}/review.
func (d *Deps) ReviewProposal(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "AI proposal review")
}

// DeleteProposal handles DELETE /api/v1/ai/proposals/{id}.
func (d *Deps) DeleteProposal(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "AI proposal deletion")
}

// GenerateEmailTemplate handles POST /api/v1/ai/generate/email-template.
func (d *Deps) GenerateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "Configure an AI provider in Settings to enable content generation")
}

// GenerateSubjectLines handles POST /api/v1/ai/generate/subject-lines.
func (d *Deps) GenerateSubjectLines(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "Configure an AI provider in Settings to enable content generation")
}

// GenerateLandingPageContent handles POST /api/v1/ai/generate/landing-page-content.
func (d *Deps) GenerateLandingPageContent(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "Configure an AI provider in Settings to enable content generation")
}

// GeneratePersonalization handles POST /api/v1/ai/generate/personalization.
func (d *Deps) GeneratePersonalization(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "Configure an AI provider in Settings to enable content generation")
}

// ResearchTargetOrg handles POST /api/v1/ai/research/target-org.
func (d *Deps) ResearchTargetOrg(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "AI research capabilities not yet available")
}

// ResearchIndustryTemplates handles POST /api/v1/ai/research/industry-templates.
func (d *Deps) ResearchIndustryTemplates(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "AI research capabilities not yet available")
}

// GetResearch handles GET /api/v1/ai/research/{id}.
func (d *Deps) GetResearch(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "AI research capabilities not yet available")
}

// ListResearch handles GET /api/v1/ai/research.
func (d *Deps) ListResearch(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, r, "AI research capabilities not yet available")
}

func notImplemented(w http.ResponseWriter, r *http.Request, msg string) {
	correlationID := middleware.GetCorrelationID(r.Context())
	response.Error(w, "NOT_IMPLEMENTED", msg, http.StatusNotImplemented, correlationID)
}
