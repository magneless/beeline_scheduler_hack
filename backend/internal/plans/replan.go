package plans

import (
	"context"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

func (service *Service) Replan(ctx context.Context, input contracts.ReplanRequest) (contracts.PlanResult, error) {
	policy := dispatchPolicy{key: input.OptionKey}
	if policy.key == "" {
		policy.key = "strict"
	}
	switch policy.key {
	case "strict", "original", "remove_unavailable":
	case "reserve":
		policy.reserve = true
	case "late_emergency":
		if !eventIncludes(input.Event, contracts.EventUrgentOrderAdded) {
			return contracts.PlanResult{}, contracts.InvalidInput("late emergency option requires an emergency event", nil)
		}
		strictInput := input
		strictInput.OptionKey = "strict"
		strict, err := service.Replan(ctx, strictInput)
		if err != nil {
			return contracts.PlanResult{}, err
		}
		policy.lateIDs = lateEligible(strict, nil)
		if len(policy.lateIDs) == 0 {
			strict.Draft.OptionKey = "late_emergency"
			return strict, nil
		}
	default:
		return contracts.PlanResult{}, contracts.InvalidInput("unsupported plan option", nil)
	}
	if policy.key == "original" && (eventIncludes(input.Event, contracts.EventUrgentOrderAdded) || eventIncludes(input.Event, contracts.EventEngineerUnavailable)) {
		return contracts.PlanResult{}, contracts.InvalidInput("Исходный маршрут недоступен при аварии или недоступности инженера", nil)
	}
	return service.replanWithPolicy(ctx, input, policy)
}

func (service *Service) replanWithPolicy(ctx context.Context, input contracts.ReplanRequest, policy dispatchPolicy) (contracts.PlanResult, error) {
	progress.Report(ctx, "preparing", "Читаем план и события")
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
	mode, err := service.resolveMode(input.SolveMode, &base)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	if base.ID != input.BasePlanID || base.ScenarioID != input.ScenarioID || base.SnapshotRevision != input.SnapshotRevision {
		return contracts.PlanResult{}, contracts.InvalidInput("base plan does not match requested scenario and revision", map[string]any{"base_plan_id": input.BasePlanID})
	}
	if err := validateFinalPlan(snapshot, base.Routes, base.Unassigned, base.CancelledOrderIDs, base.Lateness); err != nil {
		return contracts.PlanResult{}, err
	}
	dayStart, dayEnd, err := localDayBounds(snapshot)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	if input.Event.OccurredAt.Before(dayStart) || !input.Event.OccurredAt.Before(dayEnd) || (input.Event.OccurredAt.Before(base.AsOf) && !workFactsOnly(input.Event)) {
		return contracts.PlanResult{}, contracts.InvalidInput("event time is outside the plan day or precedes base plan as_of", map[string]any{"event_id": input.Event.ID})
	}
	if result, handled, err := service.recordUnassignedCancellation(ctx, snapshot, base, input.Event); handled {
		return result, err
	}
	if result, handled, err := service.recordWorkStatus(ctx, snapshot, base, input.Event, mode); handled {
		return result, err
	}
	if result, handled, err := service.recordPendingFacts(ctx, snapshot, base, input.Event, mode); handled {
		return result, err
	}

	target, replay, appliedEvent, newlyCancelled, err := service.prepareEvents(ctx, snapshot, base, input.Event)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	if err := requireHistoricalCompletions(target); err != nil {
		return contracts.PlanResult{}, err
	}
	input.Event.OccurredAt = laterTime(base.AsOf, input.Event.OccurredAt)
	target.Revision = input.SnapshotRevision + 1
	// A disabled engineer cannot retain a future visit, even while travelling.
	for _, eng := range target.Engineers {
		if !eng.Available {
			delete(replay.lockedOrders, replay.enrouteOrders[eng.ID])
			delete(replay.enrouteOrders, eng.ID)
		}
	}
	if eventIncludes(input.Event, contracts.EventUrgentOrderAdded) {
		for _, orderID := range replay.enrouteOrders {
			delete(replay.lockedOrders, orderID)
		}
	} else if err := service.protectEnRoute(ctx, target, base, &replay, input.Event); err != nil {
		return contracts.PlanResult{}, err
	}
	if policy.reserve {
		policy.wasAvailable = map[string]bool{}
		for _, engineer := range target.Engineers {
			policy.wasAvailable[engineer.ID] = engineer.Available
		}
		for i := range target.Engineers {
			if target.Engineers[i].Reserve && !target.Engineers[i].Available {
				target.Engineers[i].Available = true
			}
		}
	}
	if err := validateSnapshot(target, input.ScenarioID, target.Revision); err != nil {
		return contracts.PlanResult{}, err
	}

	if policy.key == "original" || policy.key == "remove_unavailable" {
		onlyUnavailable := true
		for _, event := range eventList(input.Event) {
			if event.Type != contracts.EventEngineerUnavailable {
				onlyUnavailable = false
			}
		}
		if policy.key == "remove_unavailable" && onlyUnavailable {
			return removeUnavailable(target, base, replay, appliedEvent, mode)
		}
		calculationEvent := appliedEvent
		calculationEvent.OccurredAt = input.Event.OccurredAt
		result, err := service.replanOriginal(ctx, target, base, replay, calculationEvent, newlyCancelled, mode)
		result.AppliedEvent = &appliedEvent
		result.Draft.OptionKey = policy.key
		return result, err
	}
	if ordinaryOnly(input.Event) {
		result, err := service.replanOrdinary(ctx, target, base, replay, appliedEvent, mode, policy)
		if err == nil && input.OptionKey == "" {
			result.Draft.OptionKey = ""
			result.Draft.DeferredOrderIDs = nil
		}
		return result, err
	}
	statusOnly := true
	changedEngineers := map[string]bool{}
	for _, ev := range eventList(input.Event) {
		if ev.Type != contracts.EventOrderStatusChanged {
			statusOnly = false
			break
		}
		changedEngineers[contracts.DecodePayload(ev.Payload).EngineerID] = true
	}
	if statusOnly {
		for _, saved := range base.Routes {
			if changedEngineers[saved.EngineerID] {
				continue
			}
			for _, visit := range saved.Visits {
				replay.lockedOrders[visit.OrderID] = struct{}{}
			}
			index := -1
			for i := range replay.routes {
				if replay.routes[i].EngineerID == saved.EngineerID {
					index = i
					break
				}
			}
			preserved := clonePlanRoutes([]contracts.Route{saved})[0]
			if index < 0 {
				replay.routes = append(replay.routes, preserved)
			} else {
				replay.routes[index] = preserved
			}
		}
	}

	prepareSnapshot := cloneSnapshot(target)
	lateAllowed := map[string]bool{}
	for id := range policy.lateIDs {
		lateAllowed[id] = true
	}
	for _, item := range base.Lateness {
		lateAllowed[item.OrderID] = true
	}
	prepareSnapshot.Orders = widenEmergencyWindows(prepareSnapshot.Orders, lateAllowed, target.Engineers)
	remaining, expired := prepareRemainingOrders(prepareSnapshot, replay.lockedOrders, input.Event.OccurredAt)
	previouslyUnassigned := map[string]bool{}
	for _, item := range base.Unassigned {
		previouslyUnassigned[item.OrderID] = true
	}
	filtered := remaining[:0]
	for _, order := range remaining {
		if !previouslyUnassigned[order.ID] {
			filtered = append(filtered, order)
		}
	}
	remaining = filtered
	if len(previouslyUnassigned) > 0 {
		keptExpired := expired[:0]
		for _, item := range expired {
			if !previouslyUnassigned[item.OrderID] {
				keptExpired = append(keptExpired, item)
			}
		}
		expired = keptExpired
	}
	retainedUnassigned := []contracts.UnassignedOrder{}
	for _, item := range base.Unassigned {
		order := orderMap(target.Orders)[item.OrderID]
		if order.ID != "" && order.Status != contracts.OrderStatusCancelled {
			retainedUnassigned = append(retainedUnassigned, item)
		}
	}
	engineers := availableEngineers(target)
	eligible := make([]contracts.Engineer, 0, len(engineers))
	for _, eng := range engineers {
		if !replay.blocked[eng.ID] && (!statusOnly || changedEngineers[eng.ID]) {
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
		progress.Report(ctx, "matrix", "Получаем время в пути")
		matrix, err := service.geo.BuildMatrix(ctx, contracts.MatrixRequest{Locations: locations, Profiles: profiles, GeoContextID: replay.geoContextID})
		if err != nil {
			return contracts.PlanResult{}, dependencyError("build replanning travel matrix", err)
		}
		if err := validateMatrix(matrix, locations, profiles); err != nil {
			return contracts.PlanResult{}, err
		}
		solveRequest := contracts.SolveRequest{
			EmergencyFirst:         policy.key == "late_emergency",
			Mode:                   mode,
			Orders:                 remaining,
			Engineers:              engineers,
			EngineerStates:         states,
			AlreadyUsedEngineerIDs: append([]string(nil), replay.usedEngineers...),
			TravelMatrix:           matrix,
			TimeLimitMS:            service.timeLimitMS,
		}
		progress.Report(ctx, "solving", "Рассчитываем маршрут")
		solveResult, err = service.planner.Solve(ctx, cloneSolveRequest(solveRequest))
		if err != nil {
			return contracts.PlanResult{}, dependencyError("solve replanned future", err)
		}
		if err := validateSolveResult(solveRequest, solveResult); err != nil {
			return contracts.PlanResult{}, err
		}
		future := renameCollidingFutureLegs(solveResult.Routes, replay.routes, input.Event.ID)
		progress.Report(ctx, "geometry", "Загружаем маршруты на карту")
		future, err = service.addGeometry(ctx, future, engineers, locations, matrix.GeoContextID)
		if err != nil {
			return contracts.PlanResult{}, err
		}
		solveResult.Routes = future
	}

	unassigned := append(append(retainedUnassigned, expired...), solveResult.Unassigned...)
	unassigned = nonNil(unassigned)
	routes := mergeRoutes(replay.routes, solveResult.Routes)
	var recruited []string
	for i := range target.Engineers {
		eng := &target.Engineers[i]
		if !policy.reserve || !eng.Reserve || snapshot.Engineers[i].Available {
			continue
		}
		used := false
		for _, route := range routes {
			if route.EngineerID == eng.ID && len(route.Visits) > 0 {
				used = true
				break
			}
		}
		if !used {
			eng.Available = false
		} else {
			recruited = append(recruited, eng.ID)
			eng.Reserve = false
		}
	}
	resetReassigned(&target, routes)
	cancelledIDs := mergeCancelled(base.CancelledOrderIDs, newlyCancelled)
	lateness := carryAcceptedLateness(target, routes, base.Lateness)
	if err := validateFinalPlan(target, routes, unassigned, cancelledIDs, lateness); err != nil {
		return contracts.PlanResult{}, err
	}
	cancelledSet := make(map[string]struct{}, len(newlyCancelled))
	for _, orderID := range newlyCancelled {
		cancelledSet[orderID] = struct{}{}
	}
	basePlanID := input.BasePlanID
	draft := contracts.PlanDraft{
		OptionKey:         input.OptionKey,
		Lateness:          lateness,
		SolveMode:         mode,
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
	if input.OptionKey != "" {
		draft.DeferredOrderIDs = deferredOrderIDs(unassigned)
	}
	draft.ReserveEngineerIDs = recruited
	sort.Strings(draft.ReserveEngineerIDs)
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
			draft.Issues = append(draft.Issues, contracts.Issue{EntityID: &id, Code: "EXECUTION_STATE_REQUIRED", Message: "Плановое время истекло. Подтвердите фактическое завершение, когда работа закончится."})
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
	if mode == contracts.SolveModeBaseline {
		draft.Issues = append(draft.Issues, contracts.Issue{Code: "BASELINE_ONLY", Message: "Выбран базовый алгоритм планирования"})
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
