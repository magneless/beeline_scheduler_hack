package plans

import "github.com/magneless/beeline_scheduler_hack/contracts"

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
		if !ok || a.EngineerID != o.Execution.EngineerID {
			o.Status = contracts.OrderStatusActive
			o.Execution = nil
		}
	}
}
