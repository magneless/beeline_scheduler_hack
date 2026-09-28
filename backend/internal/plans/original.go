package plans

import (
	"context"
	"fmt"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

// replanOriginal keeps every still-feasible appointment at its planned start.
// A missing visit is surfaced for dispatcher action rather than silently moved.
func (service *Service) replanOriginal(ctx context.Context, target contracts.Snapshot, base contracts.Plan, replay replayResult, event contracts.Event, newlyCancelled []string, mode contracts.SolveMode) (contracts.PlanResult, error) {
	orders := orderMap(target.Orders)
	engineers := engineerMap(target.Engineers)
	payload := contracts.DecodePayload(event.Payload)
	preserveOnStart := event.Type == contracts.EventOrderStatusChanged && payload.Status == contracts.OrderStatusInProgress
	retained := []contracts.UnassignedOrder{}
	seenUnassigned := map[string]bool{}
	addUnassigned := func(id string, code contracts.UnassignedReason, message string) {
		if seenUnassigned[id] {
			return
		}
		seenUnassigned[id] = true
		retained = append(retained, contracts.UnassignedOrder{OrderID: id, ReasonCode: code, Message: message})
	}
	for _, item := range base.Unassigned {
		if orders[item.OrderID].ID != "" && orders[item.OrderID].Status != contracts.OrderStatusCancelled {
			addUnassigned(item.OrderID, item.ReasonCode, item.Message)
		}
	}
	baseAssigned := map[string]bool{}
	for _, route := range base.Routes {
		for _, visit := range route.Visits {
			baseAssigned[visit.OrderID] = true
		}
	}
	for _, order := range activeValidOrders(target) {
		if !baseAssigned[order.ID] && !seenUnassigned[order.ID] {
			addUnassigned(order.ID, contracts.ReasonNoFeasibleInsertion, "Новая заявка ожидает решения диспетчера")
		}
	}
	states := []contracts.EngineerState{}
	neededOrders := []contracts.Order{}
	for _, route := range base.Routes {
		state, ok := replay.states[route.EngineerID]
		if !ok {
			continue
		}
		states = append(states, state)
		for _, visit := range route.Visits {
			if _, locked := replay.lockedOrders[visit.OrderID]; locked {
				continue
			}
			order := orders[visit.OrderID]
			if order.ID != "" && order.Status != contracts.OrderStatusCancelled {
				neededOrders = append(neededOrders, order)
			}
		}
	}
	locations, err := relevantLocations(target, neededOrders, states)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	profiles := profilesFor(target.Engineers)
	var matrix contracts.TravelMatrix
	if len(neededOrders) > 0 {
		matrix, err = service.geo.BuildMatrix(ctx, contracts.MatrixRequest{Locations: locations, Profiles: profiles, GeoContextID: replay.geoContextID})
		if err != nil {
			return contracts.PlanResult{}, dependencyError("build retained-route matrix", err)
		}
		if err := validateMatrix(matrix, locations, profiles); err != nil {
			return contracts.PlanResult{}, err
		}
	}
	future := []contracts.Route{}
	for _, saved := range base.Routes {
		state, exists := replay.states[saved.EngineerID]
		if !exists {
			continue
		}
		engineer := engineers[saved.EngineerID]
		route := contracts.Route{EngineerID: saved.EngineerID, StartLocationID: state.StartLocationID, StartAt: state.AvailableFrom, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}}
		location, available := state.StartLocationID, state.AvailableFrom
		for index, old := range saved.Visits {
			if _, locked := replay.lockedOrders[old.OrderID]; locked {
				continue
			}
			order := orders[old.OrderID]
			if order.ID == "" || order.Status == contracts.OrderStatusCancelled {
				continue
			}
			if !engineer.Available || replay.blocked[engineer.ID] && !preserveOnStart {
				addUnassigned(order.ID, contracts.ReasonNoAvailableEngineer, "Инженер недоступен для оставшихся работ")
				continue
			}
			cell, cellErr := matrixCell(matrix, engineer.Transport, location, order.LocationID)
			if cellErr != nil || !cell.Reachable || cell.DurationSec == nil || cell.DistanceM == nil {
				addUnassigned(order.ID, contracts.ReasonNoReachableRoute, "Нет доступного пути к заявке")
				continue
			}
			depart := old.ArrivalAt.Add(-time.Duration(*cell.DurationSec) * time.Second)
			if index < len(saved.Legs) && saved.Legs[index].StartAt.After(depart) {
				depart = saved.Legs[index].StartAt
			}
			if depart.Before(available) {
				depart = available
			}
			if depart.Before(event.OccurredAt) {
				depart = event.OccurredAt
			}
			arrival := depart.Add(time.Duration(*cell.DurationSec) * time.Second)
			if arrival.Before(old.ArrivalAt) {
				arrival = old.ArrivalAt
			}
			start := old.StartAt
			if arrival.After(start) {
				start = arrival
			}
			end := start.Add(time.Duration(order.ServiceSec) * time.Second)
			latest := order.Window.End
			for _, item := range base.Lateness {
				if item.OrderID == order.ID && old.StartAt.After(latest) {
					latest = old.StartAt
				}
			}
			if start.After(latest) || end.After(engineer.Shift.End) {
				addUnassigned(order.ID, contracts.ReasonNoFeasibleSlot, "Сохранить время визита после события невозможно")
				continue
			}
			leg := contracts.Leg{ID: fmt.Sprintf("retained-%s-%s-%d", event.ID, engineer.ID, index), FromLocationID: location, ToLocationID: order.LocationID, StartAt: depart, EndAt: depart.Add(time.Duration(*cell.DurationSec) * time.Second), DistanceM: *cell.DistanceM, GeoContextID: matrix.GeoContextID}
			if leg.EndAt.Before(arrival) {
				leg.StartAt = arrival.Add(-time.Duration(*cell.DurationSec) * time.Second)
				leg.EndAt = arrival
			}
			route.Legs = append(route.Legs, leg)
			route.Visits = append(route.Visits, contracts.Visit{OrderID: order.ID, ArrivalAt: leg.EndAt, StartAt: start, EndAt: end})
			location, available = order.LocationID, end
		}
		if len(route.Visits) > 0 {
			future = append(future, route)
		}
	}
	if len(future) > 0 {
		future, err = service.addGeometry(ctx, future, target.Engineers, locations, matrix.GeoContextID)
		if err != nil {
			return contracts.PlanResult{}, err
		}
	}
	routes := mergeRoutes(replay.routes, future)
	resetReassigned(&target, routes)
	cancelled := mergeCancelled(base.CancelledOrderIDs, newlyCancelled)
	lateness := carryAcceptedLateness(target, routes, base.Lateness)
	if err := validateFinalPlan(target, routes, retained, cancelled, lateness); err != nil {
		return contracts.PlanResult{}, err
	}
	stock, err := equipmentRemaining(target)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	completed := []string{}
	for _, order := range target.Orders {
		if order.Status == contracts.OrderStatusCompleted {
			completed = append(completed, order.ID)
		}
	}
	metrics := calculateMetrics(routes, retained)
	metrics.CompletedCount = len(completed)
	baseID := base.ID
	key := "original"
	if event.Type == contracts.EventOrderStatusChanged {
		key = "strict"
	}
	draft := contracts.PlanDraft{OptionKey: key, Lateness: lateness, SolveMode: mode, ScenarioID: target.ScenarioID, SnapshotRevision: target.Revision, BasePlanID: &baseID, AsOf: event.OccurredAt, Routes: routes, Unassigned: retained, CancelledOrderIDs: cancelled, CompletedOrderIDs: completed, EquipmentRemaining: stock, Issues: append([]contracts.Issue{}, target.Issues...), Metrics: metrics, Changes: calculateChanges(base.Routes, routes, nil), Termination: contracts.TerminationCompleted}
	draft.DeferredOrderIDs = deferredOrderIDs(retained)
	return contracts.PlanResult{Draft: draft, TargetSnapshot: target, AppliedEvent: &event}, nil
}
