// Package search provides the global cross-entity search endpoint.
package search

import (
	"database/sql"
	"net/http"
	"strings"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// Deps holds dependencies for the search handler.
type Deps struct {
	DB *sql.DB
}

// SearchResult represents a single search hit.
type SearchResult struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// SearchResponse groups results by entity type.
type SearchResponse struct {
	Campaigns []SearchResult `json:"campaigns"`
	Targets   []SearchResult `json:"targets"`
	Templates    []SearchResult `json:"templates"`
	Domains      []SearchResult `json:"domains"`
	Applications []SearchResult `json:"applications"`
}

const maxPerType = 5

// Search handles GET /api/v1/search?q={query}&types=campaigns,targets,templates,domains.
func (d *Deps) Search(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		response.Success(w, SearchResponse{
			Campaigns:    []SearchResult{},
			Targets:      []SearchResult{},
			Templates:    []SearchResult{},
			Domains:      []SearchResult{},
			Applications: []SearchResult{},
		})
		return
	}

	typesParam := r.URL.Query().Get("types")
	wantTypes := map[string]bool{
		"campaigns":    true,
		"targets":      true,
		"templates":    true,
		"domains":      true,
		"applications": true,
	}
	if typesParam != "" {
		wantTypes = map[string]bool{}
		for _, t := range strings.Split(typesParam, ",") {
			wantTypes[strings.TrimSpace(t)] = true
		}
	}

	pattern := "%" + q + "%"
	resp := SearchResponse{
		Campaigns:    []SearchResult{},
		Targets:      []SearchResult{},
		Templates:    []SearchResult{},
		Domains:      []SearchResult{},
		Applications: []SearchResult{},
	}

	if wantTypes["campaigns"] {
		rows, err := d.DB.QueryContext(r.Context(),
			`SELECT id, name FROM campaigns WHERE name ILIKE $1 ORDER BY updated_at DESC LIMIT $2`,
			pattern, maxPerType,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sr SearchResult
				if err := rows.Scan(&sr.ID, &sr.Name); err == nil {
					sr.Type = "campaign"
					sr.Path = "/campaigns/" + sr.ID
					resp.Campaigns = append(resp.Campaigns, sr)
				}
			}
		}
	}

	if wantTypes["targets"] {
		rows, err := d.DB.QueryContext(r.Context(),
			`SELECT id, COALESCE(first_name || ' ' || last_name, email) FROM targets WHERE email ILIKE $1 OR first_name ILIKE $1 OR last_name ILIKE $1 ORDER BY updated_at DESC LIMIT $2`,
			pattern, maxPerType,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sr SearchResult
				if err := rows.Scan(&sr.ID, &sr.Name); err == nil {
					sr.Type = "target"
					sr.Path = "/campaigns"
					resp.Targets = append(resp.Targets, sr)
				}
			}
		}
	}

	if wantTypes["templates"] {
		rows, err := d.DB.QueryContext(r.Context(),
			`SELECT id, name FROM email_templates WHERE name ILIKE $1 ORDER BY updated_at DESC LIMIT $2`,
			pattern, maxPerType,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sr SearchResult
				if err := rows.Scan(&sr.ID, &sr.Name); err == nil {
					sr.Type = "template"
					sr.Path = "/email-templates/" + sr.ID
					resp.Templates = append(resp.Templates, sr)
				}
			}
		}
	}

	if wantTypes["domains"] {
		rows, err := d.DB.QueryContext(r.Context(),
			`SELECT id, domain_name FROM domain_profiles WHERE domain_name ILIKE $1 ORDER BY updated_at DESC LIMIT $2`,
			pattern, maxPerType,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sr SearchResult
				if err := rows.Scan(&sr.ID, &sr.Name); err == nil {
					sr.Type = "domain"
					sr.Path = "/engineering"
					resp.Domains = append(resp.Domains, sr)
				}
			}
		}
	}

	if wantTypes["applications"] {
		rows, err := d.DB.QueryContext(r.Context(),
			`SELECT id, name FROM landing_pages WHERE name ILIKE $1 ORDER BY updated_at DESC LIMIT $2`,
			pattern, maxPerType,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sr SearchResult
				if err := rows.Scan(&sr.ID, &sr.Name); err == nil {
					sr.Type = "application"
					sr.Path = "/builder/" + sr.ID
					resp.Applications = append(resp.Applications, sr)
				}
			}
		}
	}

	response.Success(w, resp)
}
