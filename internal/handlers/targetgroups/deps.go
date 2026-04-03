package targetgroups

import (
	"tackle/internal/repositories"
	blocklistsvc "tackle/internal/services/blocklist"
	targetgroupsvc "tackle/internal/services/targetgroup"
)

// Deps holds the handler dependencies.
type Deps struct {
	GroupSvc     *targetgroupsvc.Service
	BlocklistSvc *blocklistsvc.Service
	CanaryRepo   *repositories.CanaryTargetRepository
}
