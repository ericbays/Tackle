// Package landingpages provides HTTP handlers for landing page project management.
package landingpages

import (
	"tackle/internal/compiler"
	"tackle/internal/compiler/hosting"
	lpsvc "tackle/internal/services/landingpage"
)

// Deps holds handler dependencies.
type Deps struct {
	Svc    *lpsvc.Service
	Engine *compiler.CompilationEngine
	AppMgr *hosting.AppManager
}
