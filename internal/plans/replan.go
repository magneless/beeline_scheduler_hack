package plans

import (
	"context"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

func (service *Service) Replan(ctx context.Context, input contracts.ReplanRequest) (contracts.PlanResult, error) {
	if err := ctx.Err(); err != nil {
		return contracts.PlanResult{}, err
	}
	if input.RequestID == "" || input.ScenarioID == "" || input.SnapshotRevision <= 0 || input.BasePlanID == "" || input.Event.ID == "" || input.Event.OccurredAt.IsZero() {
		return contracts.PlanResult{}, contracts.InvalidInput("request_id, scenario_id, snapshot_revision, base_plan_id and complete event are required", nil)
	}
	snapshot, err := service.data.GetSnapshot(ctx, input.ScenarioID, input.SnapshotRevision)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("get snapshot", err)
	}
	if err := validateSnapshot(snapshot, input.ScenarioID, input.SnapshotRevision); err != nil {
		return contracts.PlanResult{}, err
	}
	base, err := service.data.GetPlan(ctx, input.BasePlanID)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("get base plan", err)
	}
	if base.ID != input.BasePlanID || base.ScenarioID != input.ScenarioID || base.SnapshotRevision != input.SnapshotRevision {
		return contracts.PlanResult{}, contracts.InvalidInput("base plan does not match requested scenario and revision", map[string]any{"base_plan_id": input.BasePlanID})
	}
	if err := validateFinalPlan(snapshot, base.Routes, base.Unassigned, base.CancelledOrderIDs, base.CompletedOrderIDs); err != nil {
		return contracts.PlanResult{}, err
	}
	dayStart, dayEnd, err := localDayBounds(snapshot)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	if input.Event.OccurredAt.Before(dayStart) || !input.Event.OccurredAt.Before(dayEnd) || input.Event.OccurredAt.Before(base.AsOf) {
		return contracts.PlanResult{}, contracts.InvalidInput("event time is outside the plan day or precedes base plan as_of", map[string]any{"event_id": input.Event.ID})
	}

	target := cloneSnapshot(snapshot)
	appliedEvent, err := service.applyEvent(ctx, &target, base, input.Event)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	target.Revision = input.SnapshotRevision + 1
	if err := validateSnapshot(target, input.ScenarioID, target.Revision); err != nil {
		return contracts.PlanResult{}, err
	}
	replay, err := service.replayAt(ctx, &target, base, input.Event)
	if err != nil {
		return contracts.PlanResult{}, err
	}

	remaining, expired := prepareRemainingOrders(target, replay.lockedOrders, input.Event.OccurredAt)
	available := availableEngineers(target)
	engineers := make([]contracts.Engineer, 0, len(available))
	states := make([]contracts.EngineerState, 0, len(engineers))
	for _, engineer := range available {
		state, exists := replay.states[engineer.ID]
		if !exists {
			continue
		}
		engineers = append(engineers, engineer)
		states = append(states, state)
	}

	solveResult := contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}
	if len(remaining) > 0 && len(engineers) == 0 {
		for _, order := range remaining {
			solveResult.Unassigned = append(solveResult.Unassigned, contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: contracts.ReasonNoAvailableEngineer, Message: "Нет доступных инженеров"})
		}
	} else if len(remaining) > 0 {
		locations, err := relevantLocations(target, remaining, states)
		if err != nil {
			return contracts.PlanResult{}, err
		}
		profiles := profilesFor(engineers)
		matrix, err := service.geo.BuildMatrix(ctx, contracts.MatrixRequest{Locations: locations, Profiles: profiles, GeoContextID: replay.geoContextID})
		if err != nil {
			return contracts.PlanResult{}, dependencyError("build replanning travel matrix", err)
		}
		if err := validateMatrix(matrix, locations, profiles); err != nil {
			return contracts.PlanResult{}, err
		}
		solveRequest := contracts.SolveRequest{
			Mode:                   contracts.SolveModeOptimized,
			Orders:                 remaining,
			Engineers:              engineers,
			EngineerStates:         states,
			AlreadyUsedEngineerIDs: append([]string(nil), replay.usedEngineers...),
			TravelMatrix:           matrix,
			TimeLimitMS:            service.timeLimitMS,
		}
		solveResult, err = service.planner.Solve(ctx, cloneSolveRequest(solveRequest))
		if err != nil {
			return contracts.PlanResult{}, dependencyError("solve replanned future", err)
		}
		if err := validateSolveResult(solveRequest, solveResult); err != nil {
			return contracts.PlanResult{}, err
		}
		future := renameCollidingFutureLegs(solveResult.Routes, replay.routes, input.Event.ID)
		future, err = service.addGeometry(ctx, future, engineers, locations, matrix.GeoContextID)
		if err != nil {
			return contracts.PlanResult{}, err
		}
		solveResult.Routes = future
	}
	resetReassignedExecution(&target, solveResult.Routes)
	if err := validateSnapshot(target, input.ScenarioID, target.Revision); err != nil {
		return contracts.PlanResult{}, err
	}

	unassigned := append(expired, solveResult.Unassigned...)
	unassigned = nonNil(unassigned)
	routes := mergeRoutes(replay.routes, solveResult.Routes)
	cancelledIDs := statusOrderIDs(target, contracts.OrderStatusCancelled)
	completedIDs := statusOrderIDs(target, contracts.OrderStatusCompleted)
	if err := validateFinalPlan(target, routes, unassigned, cancelledIDs, completedIDs); err != nil {
		return contracts.PlanResult{}, err
	}
	cancelledSet := make(map[string]struct{}, len(cancelledIDs))
	for _, orderID := range cancelledIDs {
		cancelledSet[orderID] = struct{}{}
	}
	statusChanged := map[string]struct{}{}
	if input.Event.Type == contracts.EventOrderStatusChanged {
		statusChanged[input.Event.Payload.OrderID] = struct{}{}
	}
	equipment, err := equipmentRemaining(target)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	basePlanID := input.BasePlanID
	draft := contracts.PlanDraft{
		ScenarioID:         target.ScenarioID,
		SnapshotRevision:   target.Revision,
		BasePlanID:         &basePlanID,
		AsOf:               input.Event.OccurredAt,
		Routes:             routes,
		Unassigned:         unassigned,
		CancelledOrderIDs:  cancelledIDs,
		CompletedOrderIDs:  completedIDs,
		EquipmentRemaining: equipment,
		Issues:             nonNil(append([]contracts.Issue(nil), target.Issues...)),
		Metrics:            calculateMetrics(routes, unassigned, cancelledIDs, completedIDs),
		BaselineMetrics:    nil,
		Changes:            calculateChanges(base.Routes, routes, cancelledSet, statusChanged),
		Termination:        solveResult.Termination,
	}
	return contracts.PlanResult{Draft: draft, TargetSnapshot: target, AppliedEvent: &appliedEvent}, nil
}

func prepareRemainingOrders(snapshot contracts.Snapshot, locked map[string]struct{}, eventAt time.Time) ([]contracts.Order, []contracts.UnassignedOrder) {
	remaining := make([]contracts.Order, 0)
	expired := make([]contracts.UnassignedOrder, 0)
	for _, order := range activeValidOrders(snapshot) {
		if _, exists := locked[order.ID]; exists {
			continue
		}
		lowerBound := eventAt
		if order.ReceivedAt.After(lowerBound) {
			lowerBound = order.ReceivedAt
		}
		if order.Window.End.Before(lowerBound) {
			expired = append(expired, contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "К моменту события временное окно уже завершилось"})
			continue
		}
		copyOrder := cloneOrders([]contracts.Order{order})[0]
		if copyOrder.Window.Start.Before(lowerBound) {
			copyOrder.Window.Start = lowerBound
		}
		remaining = append(remaining, copyOrder)
	}
	return remaining, expired
}

func resetReassignedExecution(snapshot *contracts.Snapshot, routes []contracts.Route) {
	assigned := assignments(routes)
	for index := range snapshot.Orders {
		order := &snapshot.Orders[index]
		if (order.Status != contracts.OrderStatusSent && order.Status != contracts.OrderStatusEnRoute) || order.Execution == nil {
			continue
		}
		assignment, exists := assigned[order.ID]
		if exists && assignment.EngineerID == order.Execution.EngineerID {
			continue
		}
		order.Status = contracts.OrderStatusActive
		order.Execution = nil
	}
}
