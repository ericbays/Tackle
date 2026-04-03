package targets

import targetsvc "tackle/internal/services/target"

// Deps holds the handler dependencies.
type Deps struct {
	Svc *targetsvc.Service
}
