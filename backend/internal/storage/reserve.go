package storage

import c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"

// Reserve is fixed when the first working plan is accepted. Completing or
// cancelling the last visit later in the day never creates a new reserve crew.
func initialReserve(target c.Snapshot, routes []c.Route) (c.Snapshot, bool) {
	used := map[string]bool{}
	for _, route := range routes {
		used[route.EngineerID] = len(route.Visits) > 0
	}
	target.Engineers = append([]c.Engineer(nil), target.Engineers...)
	changed := false
	for i := range target.Engineers {
		e := &target.Engineers[i]
		eligible := e.Available || e.Reserve
		reserve, available := eligible && !used[e.ID], eligible && used[e.ID]
		changed = changed || e.Reserve != reserve || e.Available != available
		e.Reserve, e.Available = reserve, available
	}
	if changed {
		target.ReserveInitialized = true
		target.Revision++
	}
	return target, changed
}
