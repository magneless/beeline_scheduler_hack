package plans

import (
	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

// Removing a disabled crew needs no new routing. Other crews retain every
// appointment and road segment; this crew retains only confirmed history.
func removeUnavailable(target contracts.Snapshot, base contracts.Plan, replay replayResult, event contracts.Event, mode contracts.SolveMode) (contracts.PlanResult, error) {
	routes := []contracts.Route{}
	unassigned := append([]contracts.UnassignedOrder{}, base.Unassigned...)
	orders := orderMap(target.Orders)
	engineers := engineerMap(target.Engineers)
	for _, saved := range base.Routes {
		if engineers[saved.EngineerID].Available {
			routes = append(routes, clonePlanRoutes([]contracts.Route{saved})...)
			continue
		}
		for _, route := range replay.routes {
			if route.EngineerID == saved.EngineerID {
				routes = append(routes, route)
			}
		}
		for _, visit := range saved.Visits {
			order := orders[visit.OrderID]
			if order.Status == contracts.OrderStatusCancelled || order.Execution != nil && order.Execution.StartedAt != nil {
				continue
			}
			unassigned = append(unassigned, contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: contracts.ReasonNoAvailableEngineer, Message: "Бригада недоступна. Заявка снята с маршрута и оставлена на разбор диспетчеру."})
		}
	}
	resetReassigned(&target, routes)
	if err := validateFinalPlan(target, routes, unassigned, base.CancelledOrderIDs, base.Lateness); err != nil {
		return contracts.PlanResult{}, err
	}
	stock, err := equipmentRemaining(target)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	draft := base.PlanDraft
	draft.OptionKey, draft.SolveMode = "remove_unavailable", mode
	draft.BasePlanID, draft.AsOf, draft.SnapshotRevision = &base.ID, event.OccurredAt, target.Revision
	draft.Routes, draft.Unassigned = routes, unassigned
	draft.DeferredOrderIDs = deferredOrderIDs(unassigned)
	draft.ReserveEngineerIDs, draft.BaselineMetrics = nil, nil
	draft.EquipmentRemaining, draft.Metrics = stock, calculateMetrics(routes, unassigned)
	draft.Metrics.CompletedCount = len(draft.CompletedOrderIDs)
	draft.Changes, draft.Termination = calculateChanges(base.Routes, routes, nil), contracts.TerminationCompleted
	return contracts.PlanResult{Draft: draft, TargetSnapshot: target, AppliedEvent: &event}, nil
}
