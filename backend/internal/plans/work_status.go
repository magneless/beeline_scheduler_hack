package plans

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

// Starting work only records execution, even when its forecast exceeds the
// accepted slot. Only actual completion after the accepted end requires routing.
// Preserve idle time before the next departure and other crews' appointments.
func (service *Service) recordWorkStatus(ctx context.Context, snapshot contracts.Snapshot, base contracts.Plan, event contracts.Event, mode contracts.SolveMode) (contracts.PlanResult, bool, error) {
	if event.Type != contracts.EventOrderStatusChanged {
		return contracts.PlanResult{}, false, nil
	}
	payload := contracts.DecodePayload(event.Payload)
	if payload.Status != contracts.OrderStatusInProgress && payload.Status != contracts.OrderStatusCompleted {
		return contracts.PlanResult{}, false, nil
	}
	routeIndex, visitIndex := -1, -1
	for i, route := range base.Routes {
		for j, visit := range route.Visits {
			if visit.OrderID == payload.OrderID {
				routeIndex, visitIndex = i, j
			}
		}
	}
	if routeIndex < 0 {
		return contracts.PlanResult{}, false, nil
	}
	previous := base.Routes[routeIndex].Visits[visitIndex]
	if payload.Status == contracts.OrderStatusCompleted && event.OccurredAt.After(previous.EndAt) {
		return contracts.PlanResult{}, false, nil
	}

	progress.Report(ctx, "saving", "Сохраняем состояние работы")
	target := cloneSnapshot(snapshot)
	applied, _, err := service.applyEvent(ctx, &target, base, event, replayResult{})
	if err != nil {
		return contracts.PlanResult{}, true, err
	}
	target.Revision++
	if err := validateSnapshot(target, target.ScenarioID, target.Revision); err != nil {
		return contracts.PlanResult{}, true, err
	}
	routes := clonePlanRoutes(base.Routes)
	visit := &routes[routeIndex].Visits[visitIndex]
	execution := orderMap(target.Orders)[payload.OrderID].Execution
	if execution == nil || execution.StartedAt == nil {
		return contracts.PlanResult{}, true, contracts.InvalidPlan("started work has no start time", nil)
	}
	if payload.Status == contracts.OrderStatusCompleted {
		visit.StartAt = *execution.StartedAt
		visit.EndAt = event.OccurredAt
	}
	// Keep the accepted visit intact while work is in progress. Facts and the
	// duration-based forecast live in execution; the forecast must not move the
	// completion threshold or overlap the next visit in the accepted schedule.
	lateness := carryAcceptedLateness(target, routes, base.Lateness)
	if err := validateFinalPlan(target, routes, base.Unassigned, base.CancelledOrderIDs, lateness); err != nil {
		return contracts.PlanResult{}, true, err
	}
	stock, err := equipmentRemaining(target)
	if err != nil {
		return contracts.PlanResult{}, true, err
	}
	completed := []string{}
	for _, order := range target.Orders {
		if order.Status == contracts.OrderStatusCompleted {
			completed = append(completed, order.ID)
		}
	}
	draft := base.PlanDraft
	draft.OptionKey = "strict"
	draft.ReserveEngineerIDs = nil
	draft.BaselineMetrics = nil
	draft.DeferredOrderIDs = deferredOrderIDs(base.Unassigned)
	draft.SolveMode = mode
	draft.SnapshotRevision = target.Revision
	draft.BasePlanID = &base.ID
	draft.AsOf = laterTime(base.AsOf, event.OccurredAt)
	draft.Routes = routes
	draft.Lateness = lateness
	draft.CompletedOrderIDs = completed
	draft.EquipmentRemaining = stock
	draft.Metrics = calculateMetrics(routes, base.Unassigned)
	draft.Metrics.CompletedCount = len(completed)
	draft.Changes = calculateChanges(base.Routes, routes, nil)
	draft.Termination = contracts.TerminationCompleted
	return contracts.PlanResult{Draft: draft, TargetSnapshot: target, AppliedEvent: &applied}, true, nil
}
