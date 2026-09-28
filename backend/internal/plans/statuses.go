package plans

import "github.com/magneless/beeline_scheduler_hack/backend/contracts"

func assignmentMap(routes []contracts.Route) map[string]*contracts.Assignment {
	out := map[string]*contracts.Assignment{}
	for id, a := range assignments(routes) {
		copy := a
		out[id] = &copy
	}
	return out
}
func resetReassigned(snapshot *contracts.Snapshot, routes []contracts.Route) {
	current := assignments(routes)
	for i := range snapshot.Orders {
		o := &snapshot.Orders[i]
		if (o.Status != contracts.OrderStatusSent && o.Status != contracts.OrderStatusEnRoute) || o.Execution == nil {
			continue
		}
		a, ok := current[o.ID]
		redirected := false
		if ok && a.EngineerID == o.Execution.EngineerID && o.Status == contracts.OrderStatusEnRoute {
			for _, route := range routes {
				if route.EngineerID != a.EngineerID {
					continue
				}
				for _, visit := range route.Visits {
					if visit.OrderID == o.ID {
						break
					}
					for _, earlier := range snapshot.Orders {
						if earlier.ID == visit.OrderID && earlier.Status != contracts.OrderStatusCompleted && (earlier.Execution == nil || earlier.Execution.StartedAt == nil) {
							redirected = true
						}
					}
					if redirected {
						break
					}
				}
			}
		}
		if !ok || a.EngineerID != o.Execution.EngineerID || redirected {
			o.Status = contracts.OrderStatusActive
			o.Execution = nil
		}
	}
}
