package plans

import "github.com/magneless/beeline_scheduler_hack/backend/contracts"

func (s *Service) buildWithoutEngineers(snapshot contracts.Snapshot, orders []contracts.Order) (contracts.PlanResult, error) {
	day, _, e := localDayBounds(snapshot)
	if e != nil {
		return contracts.PlanResult{}, e
	}
	stock, e := equipmentRemaining(snapshot)
	if e != nil {
		return contracts.PlanResult{}, e
	}
	draft := contracts.PlanDraft{ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, AsOf: day, Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, CancelledOrderIDs: []string{}, CompletedOrderIDs: []string{}, EquipmentRemaining: stock, Issues: append(append([]contracts.Issue{}, snapshot.Issues...), s.issues...), Changes: []contracts.PlanChange{}, Termination: contracts.TerminationCompleted}
	for _, o := range orders {
		draft.Unassigned = append(draft.Unassigned, contracts.UnassignedOrder{OrderID: o.ID, ReasonCode: contracts.UnassignedNoAvailableEngineer, Message: "Нет доступных инженеров"})
	}
	for _, o := range snapshot.Orders {
		if o.Status == contracts.OrderStatusCancelled {
			draft.CancelledOrderIDs = append(draft.CancelledOrderIDs, o.ID)
		}
	}
	draft.Metrics = calculateMetrics(draft.Routes, draft.Unassigned)
	metrics := draft.Metrics
	draft.BaselineMetrics = &metrics
	return contracts.PlanResult{Draft: draft, TargetSnapshot: cloneSnapshot(snapshot)}, nil
}

func actualConstraintIssues(snapshot contracts.Snapshot, routes []contracts.Route) []contracts.Issue {
	result := []contracts.Issue{}
	orders := orderMap(snapshot.Orders)
	engineers := engineerMap(snapshot.Engineers)
	for _, route := range routes {
		eng := engineers[route.EngineerID]
		for _, v := range route.Visits {
			o := orders[v.OrderID]
			if o.Execution == nil || o.Execution.StartedAt == nil {
				continue
			}
			if v.StartAt.Before(o.Window.Start) || v.StartAt.After(o.Window.End) || v.StartAt.Before(eng.Shift.Start) || v.EndAt.After(eng.Shift.End) {
				id := o.ID
				result = append(result, contracts.Issue{EntityID: &id, Code: "ACTUAL_CONSTRAINT_VIOLATION", Message: "Фактическое выполнение вышло за клиентское окно или смену"})
			}
		}
	}
	return result
}
