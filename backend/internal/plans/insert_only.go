package plans

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

// An ordinary addition freezes every old visit. Execution history is kept outside
// the inserter: it consumes the remaining stock and must not be scheduled again.
func (service *Service) replanOrdinary(ctx context.Context, target contracts.Snapshot, base contracts.Plan, replay replayResult, event contracts.Event) (contracts.PlanResult, error) {
	fresh := target.Orders[len(target.Orders)-1]
	finish := func(routes []contracts.Route, unassigned []contracts.UnassignedOrder, termination contracts.Termination, conflict bool) (contracts.PlanResult, error) {
		unassigned = append(append([]contracts.UnassignedOrder{}, base.Unassigned...), unassigned...)
		if err := validateFinalPlan(target, routes, unassigned, base.CancelledOrderIDs); err != nil {
			return contracts.PlanResult{}, err
		}
		stock, err := equipmentRemaining(target)
		if err != nil {
			return contracts.PlanResult{}, err
		}
		issues := append(append([]contracts.Issue{}, target.Issues...), service.issues...)
		if conflict {
			issues = append(issues, contracts.Issue{Code: "EXISTING_PLAN_CONFLICT", Message: "Старый план несовместим с текущим состоянием. Новая заявка сохранена без назначения; существующие визиты не изменены."})
		}
		completed := []string{}
		for _, o := range target.Orders {
			if o.Status == contracts.OrderStatusCompleted {
				completed = append(completed, o.ID)
			}
		}
		metrics := calculateMetrics(routes, unassigned)
		metrics.CompletedCount = len(completed)
		for _, r := range routes {
			for _, v := range r.Visits {
				if orderMap(target.Orders)[v.OrderID].Status == contracts.OrderStatusCancelled {
					metrics.AssignedCount--
				}
			}
		}
		draft := contracts.PlanDraft{ScenarioID: target.ScenarioID, SnapshotRevision: target.Revision, BasePlanID: ptr(base.ID), AsOf: event.OccurredAt, Routes: nonNil(routes), Unassigned: unassigned, CancelledOrderIDs: nonNil(append([]string{}, base.CancelledOrderIDs...)), CompletedOrderIDs: completed, EquipmentRemaining: stock, Issues: issues, Metrics: metrics, Changes: calculateChanges(base.Routes, routes, nil), Termination: termination}
		return contracts.PlanResult{Draft: draft, TargetSnapshot: target, AppliedEvent: &event}, nil
	}
	fallback := func(conflict bool) (contracts.PlanResult, error) {
		item := contracts.UnassignedOrder{OrderID: fresh.ID, ReasonCode: "NO_FEASIBLE_INSERTION", Message: "Нет свободного интервала для новой заявки"}
		if conflict {
			item.ReasonCode, item.Message = contracts.UnassignedBySolver, "Существующий план требует уточнения; заявка сохранена без назначения"
		}
		return finish(clonePlanRoutes(base.Routes), []contracts.UnassignedOrder{item}, contracts.TerminationCompleted, conflict)
	}
	engineers := []contracts.Engineer{}
	states := []contracts.EngineerState{}
	eligible := map[string]bool{}
	for _, eng := range availableEngineers(target) {
		if !replay.blocked[eng.ID] {
			engineers = append(engineers, eng)
			states = append(states, replay.states[eng.ID])
			eligible[eng.ID] = true
		}
	}
	orders := orderMap(target.Orders)
	fixed, history := []contracts.Route{}, []contracts.Route{}
	protected := []string{}
	solveOrders := []contracts.Order{fresh}
	for _, saved := range base.Routes {
		state := replay.states[saved.EngineerID]
		future := contracts.Route{EngineerID: saved.EngineerID, StartLocationID: state.StartLocationID, StartAt: state.AvailableFrom, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}}
		past := clonePlanRoutes([]contracts.Route{saved})[0]
		past.Visits = nil
		past.Legs = nil
		futureLegs := map[string]bool{}
		for _, visit := range saved.Visits {
			if _, actual := replay.lockedOrders[visit.OrderID]; actual {
				past.Visits = append(past.Visits, visit)
				continue
			}
			if !eligible[saved.EngineerID] {
				return fallback(true)
			}
			order := orders[visit.OrderID]
			var incoming *contracts.Leg
			for _, leg := range saved.Legs {
				if leg.ToLocationID == order.LocationID && leg.EndAt.Equal(visit.ArrivalAt) {
					copy := cloneLeg(leg)
					incoming = &copy
				}
			}
			if incoming == nil {
				return fallback(true)
			}
			if len(future.Visits) == 0 && order.Execution != nil && order.Execution.DepartedAt != nil && !incoming.StartAt.After(event.OccurredAt) {
				protected = append(protected, incoming.ID)
			}
			future.Visits = append(future.Visits, visit)
			future.Legs = append(future.Legs, *incoming)
			futureLegs[incoming.ID] = true
			solveOrders = append(solveOrders, order)
		}
		for _, leg := range saved.Legs {
			if !futureLegs[leg.ID] {
				past.Legs = append(past.Legs, cloneLeg(leg))
			}
		}
		if len(past.Legs) > 0 || len(past.Visits) > 0 {
			history = append(history, past)
		}
		if len(future.Visits) > 0 {
			fixed = append(fixed, future)
		}
	}
	if len(engineers) == 0 {
		return fallback(false)
	}
	locations, err := relevantLocations(target, solveOrders, states)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	// Protected saved legs can originate before the engineer's current position.
	wanted := map[string]bool{}
	for _, loc := range locations {
		wanted[loc.ID] = true
	}
	byLocation := locationMap(target)
	for _, r := range fixed {
		for _, leg := range r.Legs {
			if !wanted[leg.FromLocationID] {
				locations = append(locations, byLocation[leg.FromLocationID])
				wanted[leg.FromLocationID] = true
			}
		}
	}
	profiles := profilesFor(engineers)
	matrix, err := service.geo.BuildMatrix(ctx, contracts.MatrixRequest{Locations: locations, Profiles: profiles, GeoContextID: replay.geoContextID})
	if err != nil {
		return contracts.PlanResult{}, dependencyError("build insertion matrix", err)
	}
	if err := validateMatrix(matrix, locations, profiles); err != nil {
		return contracts.PlanResult{}, err
	}
	request := contracts.SolveRequest{Mode: contracts.SolveModeInsertOnly, Orders: solveOrders, Engineers: engineers, EngineerStates: states, AlreadyUsedEngineerIDs: replay.usedEngineers, TravelMatrix: matrix, FixedRoutes: fixed, ProtectedLegIDs: protected, TimeLimitMS: service.timeLimitMS}
	result, err := service.planner.Solve(ctx, cloneSolveRequest(request))
	if err != nil {
		var ce *contracts.ContractError
		if errors.As(err, &ce) && ce.Code == contracts.ErrorInvalidInput {
			field, _ := ce.Details["field"].(string)
			if strings.HasPrefix(field, "fixed_routes") || field == "protected_leg_ids" {
				return fallback(true)
			}
		}
		return contracts.PlanResult{}, dependencyError("insert ordinary order", err)
	}
	if err := validateInsertionResult(request, result); err != nil {
		return contracts.PlanResult{}, err
	}
	// Saved geometry is immutable. Fetch only new road segments.
	newSegments := []contracts.Route{}
	for _, r := range result.Routes {
		newRoute := contracts.Route{EngineerID: r.EngineerID}
		for _, leg := range r.Legs {
			if len(leg.Geometry) == 0 {
				newRoute.Legs = append(newRoute.Legs, leg)
			}
		}
		if len(newRoute.Legs) > 0 {
			newSegments = append(newSegments, newRoute)
		}
	}
	geometries, err := service.addGeometry(ctx, newSegments, engineers, locations, matrix.GeoContextID)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	newByID := map[string]contracts.Leg{}
	for _, r := range geometries {
		for _, leg := range r.Legs {
			newByID[leg.ID] = leg
		}
	}
	for ri := range result.Routes {
		for li, leg := range result.Routes[ri].Legs {
			if replacement, ok := newByID[leg.ID]; ok {
				result.Routes[ri].Legs[li] = replacement
			}
		}
	}
	result.Routes = renameCollidingFutureLegs(result.Routes, history, event.ID)
	return finish(mergeRoutes(history, result.Routes), result.Unassigned, result.Termination, false)
}

func validateInsertionResult(request contracts.SolveRequest, result contracts.SolveResult) error {
	if result.Termination != contracts.TerminationCompleted && result.Termination != contracts.TerminationTimeLimit {
		return contracts.InvalidPlan("invalid insertion termination", nil)
	}
	expected := map[string]contracts.Visit{}
	owners := map[string]string{}
	savedLegs := map[string]contracts.Leg{}
	protected := map[string]bool{}
	for _, id := range request.ProtectedLegIDs {
		protected[id] = true
	}
	for _, r := range request.FixedRoutes {
		for _, v := range r.Visits {
			expected[v.OrderID] = v
			owners[v.OrderID] = r.EngineerID
		}
		for _, leg := range r.Legs {
			savedLegs[leg.ID] = leg
		}
	}
	seen := map[string]bool{}
	seenProtected := map[string]bool{}
	orders := orderMap(request.Orders)
	engineers := engineerMap(request.Engineers)
	states := map[string]contracts.EngineerState{}
	for _, s := range request.EngineerStates {
		states[s.EngineerID] = s
	}
	for _, r := range result.Routes {
		state, ok := states[r.EngineerID]
		if !ok || r.StartLocationID != state.StartLocationID || !r.StartAt.Equal(state.AvailableFrom) || len(r.Visits) != len(r.Legs) {
			return contracts.InvalidPlan("invalid insertion route", nil)
		}
		previousLocation, previousEnd := r.StartLocationID, r.StartAt
		for i, v := range r.Visits {
			o, ok := orders[v.OrderID]
			if !ok || seen[v.OrderID] {
				return contracts.InvalidPlan("insertion repeated or invented order", nil)
			}
			seen[v.OrderID] = true
			if old, ok := expected[v.OrderID]; ok && (old != v || owners[v.OrderID] != r.EngineerID) {
				return contracts.InvalidPlan("insertion changed fixed visit", nil)
			}
			leg := r.Legs[i]
			if old, ok := savedLegs[leg.ID]; ok {
				if !reflect.DeepEqual(old, leg) {
					return contracts.InvalidPlan("insertion altered saved leg", nil)
				}
			} else {
				cell, err := matrixCell(request.TravelMatrix, engineers[r.EngineerID].Transport, previousLocation, o.LocationID)
				if err != nil {
					return err
				}
				if leg.FromLocationID != previousLocation || leg.ToLocationID != o.LocationID || leg.StartAt.Before(previousEnd) || leg.StartAt.Before(o.ReceivedAt) || !leg.EndAt.Equal(v.ArrivalAt) || !cell.Reachable || cell.DurationSec == nil || cell.DistanceM == nil || leg.EndAt.Sub(leg.StartAt) != time.Duration(*cell.DurationSec)*time.Second || leg.DistanceM != *cell.DistanceM || leg.GeoContextID != request.TravelMatrix.GeoContextID || len(leg.Geometry) != 0 {
					return contracts.InvalidPlan("invalid inserted leg", nil)
				}
			}
			if protected[leg.ID] {
				seenProtected[leg.ID] = true
			}
			previousLocation, previousEnd = o.LocationID, v.EndAt
		}
	}
	for id := range expected {
		if !seen[id] {
			return contracts.InvalidPlan("insertion omitted fixed visit", nil)
		}
	}
	for id := range protected {
		if !seenProtected[id] {
			return contracts.InvalidPlan("insertion redirected current trip", nil)
		}
	}
	for _, u := range result.Unassigned {
		if seen[u.OrderID] || orders[u.OrderID].ID == "" {
			return contracts.InvalidPlan("invalid insertion unassigned", nil)
		}
		seen[u.OrderID] = true
	}
	if len(seen) != len(request.Orders) {
		return contracts.InvalidPlan("insertion omitted new order", nil)
	}
	return nil
}

func clonePlanRoutes(routes []contracts.Route) []contracts.Route {
	out := make([]contracts.Route, len(routes))
	copy(out, routes)
	for i := range out {
		out[i].Visits = append([]contracts.Visit{}, routes[i].Visits...)
		out[i].Legs = make([]contracts.Leg, len(routes[i].Legs))
		for j, leg := range routes[i].Legs {
			out[i].Legs[j] = cloneLeg(leg)
		}
	}
	return out
}
