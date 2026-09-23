package plans

import (
	"context"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/contracts"
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
	if err := validateFinalPlan(snapshot, base.Routes, base.Unassigned, base.CancelledOrderIDs); err != nil {
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
	replay, err := service.replayAt(ctx, &target, base, input.Event)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	appliedEvent, newlyCancelled, err := service.applyEvent(ctx, &target, base, input.Event, replay)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	target.Revision = input.SnapshotRevision + 1
	if err := refreshExecution(&target, &replay, input.Event); err != nil {
		return contracts.PlanResult{}, err
	}
	if err := validateSnapshot(target, input.ScenarioID, target.Revision); err != nil {
		return contracts.PlanResult{}, err
	}

	remaining, expired := prepareRemainingOrders(target, replay.lockedOrders, input.Event.OccurredAt)
	engineers := availableEngineers(target)
	eligible := make([]contracts.Engineer, 0, len(engineers))
	for _, eng := range engineers {
		if !replay.blocked[eng.ID] {
			eligible = append(eligible, eng)
		}
	}
	engineers = eligible
	states := make([]contracts.EngineerState, 0, len(engineers))
	for _, engineer := range engineers {
		state, exists := replay.states[engineer.ID]
		if !exists {
			return contracts.PlanResult{}, contracts.InvalidPlan("replayed engineer state is missing", map[string]any{"engineer_id": engineer.ID})
		}
		states = append(states, state)
	}

	solveResult := contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}
	if len(remaining) > 0 && len(engineers) == 0 {
		for _, order := range remaining {
			solveResult.Unassigned = append(solveResult.Unassigned, contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: contracts.UnassignedNoAvailableEngineer, Message: "Нет доступных инженеров"})
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
			Mode:                   service.mode,
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

	unassigned := append(expired, solveResult.Unassigned...)
	unassigned = nonNil(unassigned)
	routes := mergeRoutes(replay.routes, solveResult.Routes)
	resetReassigned(&target, routes)
	cancelledIDs := mergeCancelled(base.CancelledOrderIDs, newlyCancelled)
	if err := validateFinalPlan(target, routes, unassigned, cancelledIDs); err != nil {
		return contracts.PlanResult{}, err
	}
	cancelledSet := make(map[string]struct{}, len(newlyCancelled))
	for _, orderID := range newlyCancelled {
		cancelledSet[orderID] = struct{}{}
	}
	basePlanID := input.BasePlanID
	draft := contracts.PlanDraft{
		ScenarioID:        target.ScenarioID,
		SnapshotRevision:  target.Revision,
		BasePlanID:        &basePlanID,
		AsOf:              input.Event.OccurredAt,
		Routes:            routes,
		Unassigned:        unassigned,
		CancelledOrderIDs: cancelledIDs,
		Issues:            nonNil(append([]contracts.Issue(nil), target.Issues...)),
		Metrics:           calculateMetrics(routes, unassigned),
		BaselineMetrics:   nil,
		Changes:           calculateChanges(base.Routes, routes, cancelledSet),
		Termination:       solveResult.Termination,
		CompletedOrderIDs: []string{},
	}
	draft.EquipmentRemaining, err = equipmentRemaining(target)
	draft.Issues = append(draft.Issues, service.issues...)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	for _, o := range target.Orders {
		if o.Status == contracts.OrderStatusCompleted {
			draft.CompletedOrderIDs = append(draft.CompletedOrderIDs, o.ID)
		}
		if o.Status == contracts.OrderStatusInProgress && replay.blocked[o.Execution.EngineerID] {
			id := o.ID
			draft.Issues = append(draft.Issues, contracts.Issue{EntityID: &id, Code: "EXECUTION_STATE_REQUIRED", Message: "Уточните ожидаемое время окончания работы"})
		}
	}
	draft.Metrics.CompletedCount = len(draft.CompletedOrderIDs)
	draft.Issues = append(draft.Issues, actualConstraintIssues(target, routes)...)
	for _, r := range routes {
		for _, v := range r.Visits {
			o := orderMap(target.Orders)[v.OrderID]
			if o.Status == contracts.OrderStatusCancelled {
				draft.Metrics.AssignedCount--
			}
		}
	}
	if input.Event.Type == contracts.EventOrderStatusChanged {
		payload := contracts.DecodePayload(input.Event.Payload)
		found := false
		for i := range draft.Changes {
			if draft.Changes[i].OrderID == payload.OrderID {
				draft.Changes[i].Reason = contracts.PlanChangeStatusChanged
				found = true
			}
		}
		if !found {
			before := assignmentMap(base.Routes)
			after := assignmentMap(routes)
			draft.Changes = append(draft.Changes, contracts.PlanChange{OrderID: payload.OrderID, Before: before[payload.OrderID], After: after[payload.OrderID], Reason: contracts.PlanChangeStatusChanged})
		}
	}
	if service.mode == contracts.SolveModeBaseline {
		draft.Issues = append(draft.Issues, contracts.Issue{Code: "BASELINE_ONLY", Message: "Использован базовый алгоритм; оптимизация ещё не подключена"})
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
		if order.Window.End.Before(eventAt) {
			expired = append(expired, contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: contracts.UnassignedNoFeasibleSlot, Message: "К моменту события временное окно уже завершилось"})
			continue
		}
		copyOrder := order
		copyOrder.RequiredSkills = append([]string(nil), order.RequiredSkills...)
		if copyOrder.Window.Start.Before(eventAt) {
			copyOrder.Window.Start = eventAt
		}
		if copyOrder.Window.Start.Before(copyOrder.ReceivedAt) {
			copyOrder.Window.Start = copyOrder.ReceivedAt
		}
		remaining = append(remaining, copyOrder)
	}
	return remaining, expired
}

func mergeCancelled(existing, added []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(added))
	result := make([]string, 0, len(existing)+len(added))
	for _, values := range [][]string{existing, added} {
		for _, value := range values {
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return nonNil(result)
}
