// Package health provides HTTP handlers for domain health checks, categorization,
// and the typosquat generator tool.
package health

import (
	catsvc "tackle/internal/services/categorization"
	healthsvc "tackle/internal/services/health"
	typosvc "tackle/internal/services/typosquat"
)

// Deps holds handler dependencies.
type Deps struct {
	HealthSvc  *healthsvc.Service
	CatSvc     *catsvc.Service
	TypoSvc    *typosvc.Service
}
