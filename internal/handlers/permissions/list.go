// Package permissions provides HTTP handlers for permission listing endpoints.
package permissions

import (
	"net/http"
	"strings"

	"tackle/internal/middleware"
	"tackle/internal/services/rbac"
	"tackle/pkg/response"
)

// permissionItem is the JSON representation of a single permission.
type permissionItem struct {
	Permission  string `json:"permission"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
}

// List handles GET /api/v1/permissions — returns all registered permissions.
func List(w http.ResponseWriter, r *http.Request) {
	_ = middleware.GetCorrelationID(r.Context()) // available if needed for errors

	all := rbac.All()
	items := make([]permissionItem, 0, len(all))
	for _, p := range all {
		parts := strings.SplitN(string(p), ":", 2)
		if len(parts) != 2 {
			continue
		}
		items = append(items, permissionItem{
			Permission: string(p),
			Resource:   parts[0],
			Action:     parts[1],
		})
	}

	response.Success(w, items)
}
