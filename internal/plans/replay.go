package plans

import (
	"context"
	"fmt"
	"github.com/magneless/beeline_scheduler_hack/contracts"
	"math"
	"sort"
	"time"
)

type replayResult struct {
	routes        []contracts.Route
	states        map[string]contracts.EngineerState
	lockedOrders  map[string]struct{}
	usedEngineers []string
	geoContextID  *string
	blocked       map[string]bool
}

// Replay only confirmed execution. Future plan timestamps never establish facts.
func (service *Service) replayAt(ctx context.Context, snapshot *contracts.Snapshot, base contracts.Plan, event contracts.Event) (replayResult, error) {
	result := replayResult{routes: []contracts.Route{}, states: map[string]contracts.EngineerState{}, lockedOrders: map[string]struct{}{}, blocked: map[string]bool{}}
	usedLocations := map[string]struct{}{}
	for _, loc := range snapshot.Locations {
		usedLocations[loc.ID] = struct{}{}
	}
	usedLegs := map[string]struct{}{}
	for _, r := range base.Routes {
		for _, l := range r.Legs {
			usedLegs[l.ID] = struct{}{}
		}
	}
	for _, eng := range snapshot.Engineers {
		at := event.OccurredAt
		if eng.Shift.Start.After(at) {
			at = eng.Shift.Start
		}
		result.states[eng.ID] = contracts.EngineerState{EngineerID: eng.ID, StartLocationID: snapshot.OfficeLocationID, AvailableFrom: at}
	}
	orders := orderMap(snapshot.Orders)
	for _, route := range base.Routes {
		state, ok := result.states[route.EngineerID]
		if !ok {
			return result, contracts.InvalidPlan("unknown route engineer", nil)
		}
		kept := contracts.Route{EngineerID: route.EngineerID, StartLocationID: route.StartLocationID, StartAt: route.StartAt, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}}
		var latest time.Time
		for _, leg := range route.Legs {
			if result.geoContextID == nil {
				result.geoContextID = ptr(leg.GeoContextID)
			} else if *result.geoContextID != leg.GeoContextID {
				return result, contracts.InvalidPlan("multiple geo contexts", nil)
			}
			confirmed := !leg.EndAt.After(base.AsOf)
			for _, o := range snapshot.Orders {
				ex := o.Execution
				if o.Status == contracts.OrderStatusCancelled && (ex == nil || ex.StartedAt == nil) {
					continue
				}
				if ex == nil || ex.EngineerID != route.EngineerID || ex.DepartedAt == nil {
					continue
				}
				if leg.ToLocationID == o.LocationID && !leg.StartAt.Before(*ex.DepartedAt) && (ex.StartedAt == nil || !leg.StartAt.After(*ex.StartedAt)) {
					confirmed = true
				}
			}
			if !confirmed || leg.StartAt.After(event.OccurredAt) {
				continue
			}
			if leg.EndAt.After(event.OccurredAt) {
				position, err := service.geo.PositionAt(ctx, contracts.PositionRequest{Leg: cloneLeg(leg), At: event.OccurredAt})
				if err != nil {
					return result, err
				}
				if err = validatePosition(leg, event.OccurredAt, position); err != nil {
					return result, err
				}
				id := uniqueID(fmt.Sprintf("event-position-%s-%s", event.ID, route.EngineerID), usedLocations)
				snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: id, Point: position.Point})
				partial := cloneLeg(leg)
				partial.ID = uniqueID(leg.ID+"-elapsed-"+event.ID, usedLegs)
				partial.ToLocationID = id
				partial.EndAt = event.OccurredAt
				partial.DistanceM = position.ElapsedDistanceM
				partial.Geometry = position.ElapsedGeometry
				kept.Legs = append(kept.Legs, partial)
				state.StartLocationID = id
				latest = event.OccurredAt
			} else {
				kept.Legs = append(kept.Legs, cloneLeg(leg))
				if !leg.EndAt.Before(latest) {
					state.StartLocationID = leg.ToLocationID
					latest = leg.EndAt
				}
			}
		}
		for _, visit := range route.Visits {
			o, ok := orders[visit.OrderID]
			if !ok {
				return result, contracts.InvalidPlan("unknown order in route", nil)
			}
			ex := o.Execution
			if ex == nil || ex.StartedAt == nil {
				continue
			}
			v := factualVisit(o, visit, event.OccurredAt)
			kept.Visits = append(kept.Visits, v)
			result.lockedOrders[o.ID] = struct{}{}
			if !v.EndAt.Before(latest) {
				state.StartLocationID = o.LocationID
				latest = v.EndAt
			}
			if o.Status == contracts.OrderStatusInProgress {
				if ex.ExpectedEndAt == nil || !ex.ExpectedEndAt.After(event.OccurredAt) {
					result.blocked[route.EngineerID] = true
				} else if ex.ExpectedEndAt.After(state.AvailableFrom) {
					state.AvailableFrom = *ex.ExpectedEndAt
				}
			}
		}
		result.states[route.EngineerID] = state
		if len(kept.Visits) > 0 || len(kept.Legs) > 0 {
			result.routes = append(result.routes, kept)
		}
	}
	if err := refreshExecution(snapshot, &result, event); err != nil {
		return result, err
	}
	return result, nil
}

func factualVisit(o contracts.Order, previous contracts.Visit, at time.Time) contracts.Visit {
	ex := o.Execution
	v := previous
	v.OrderID = o.ID
	v.StartAt = *ex.StartedAt
	if v.ArrivalAt.IsZero() || v.ArrivalAt.After(v.StartAt) {
		v.ArrivalAt = v.StartAt
	}
	v.EndAt = at
	if ex.FinishedAt != nil {
		v.EndAt = *ex.FinishedAt
	} else if ex.ExpectedEndAt != nil && ex.ExpectedEndAt.After(at) {
		v.EndAt = *ex.ExpectedEndAt
	}
	return v
}

// Refresh work facts after applying an event without repeating travel interpolation.
func refreshExecution(snapshot *contracts.Snapshot, replay *replayResult, event contracts.Event) error {
	stocks, err := equipmentRemaining(*snapshot)
	if err != nil {
		return err
	}
	payload := contracts.DecodePayload(event.Payload)
	for _, o := range snapshot.Orders {
		ex := o.Execution
		if ex == nil || ex.StartedAt == nil {
			continue
		}
		replay.lockedOrders[o.ID] = struct{}{}
		routeIndex := -1
		for i := range replay.routes {
			if replay.routes[i].EngineerID == ex.EngineerID {
				routeIndex = i
				break
			}
		}
		if routeIndex < 0 {
			replay.routes = append(replay.routes, contracts.Route{EngineerID: ex.EngineerID, StartLocationID: o.LocationID, StartAt: *ex.StartedAt, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}})
			routeIndex = len(replay.routes) - 1
		}
		r := &replay.routes[routeIndex]
		found := false
		for n, v := range r.Visits {
			if v.OrderID == o.ID {
				r.Visits[n] = factualVisit(o, v, event.OccurredAt)
				found = true
			}
		}
		if !found {
			previous := contracts.Visit{}
			for _, leg := range r.Legs {
				if leg.ToLocationID == o.LocationID && !leg.EndAt.After(*ex.StartedAt) && leg.EndAt.After(previous.ArrivalAt) {
					previous.ArrivalAt = leg.EndAt
				}
			}
			r.Visits = append(r.Visits, factualVisit(o, previous, event.OccurredAt))
		}
		sort.SliceStable(r.Visits, func(i, j int) bool { return r.Visits[i].StartAt.Before(r.Visits[j].StartAt) })
		if o.ID == payload.OrderID || o.Status == contracts.OrderStatusInProgress {
			state := replay.states[ex.EngineerID]
			state.StartLocationID = o.LocationID
			state.AvailableFrom = event.OccurredAt
			for _, eng := range snapshot.Engineers {
				if eng.ID == ex.EngineerID && eng.Shift.Start.After(state.AvailableFrom) {
					state.AvailableFrom = eng.Shift.Start
				}
			}
			replay.blocked[ex.EngineerID] = false
			if o.Status == contracts.OrderStatusInProgress {
				if ex.ExpectedEndAt == nil || !ex.ExpectedEndAt.After(event.OccurredAt) {
					replay.blocked[ex.EngineerID] = true
				} else {
					state.AvailableFrom = *ex.ExpectedEndAt
				}
			}
			replay.states[ex.EngineerID] = state
		}
	}
	for id, state := range replay.states {
		state.EquipmentAvailable = stocks[id]
		replay.states[id] = state
	}
	replay.usedEngineers = []string{}
	for _, r := range replay.routes {
		if len(r.Visits) > 0 || len(r.Legs) > 0 {
			replay.usedEngineers = append(replay.usedEngineers, r.EngineerID)
		}
	}
	sort.Strings(replay.usedEngineers)
	return nil
}

func validatePosition(leg contracts.Leg, at time.Time, position contracts.PositionResult) error {
	duration := int64(at.Sub(leg.StartAt) / time.Second)
	if position.ElapsedDurationSec != duration || position.ElapsedDurationSec < 0 || position.ElapsedDurationSec > int64(leg.EndAt.Sub(leg.StartAt)/time.Second) {
		return contracts.InvalidPlan("PositionAt returned an invalid elapsed duration", map[string]any{"leg_id": leg.ID})
	}
	if position.ElapsedDistanceM < 0 || position.ElapsedDistanceM > leg.DistanceM || len(position.ElapsedGeometry) == 0 {
		return contracts.InvalidPlan("PositionAt returned an invalid distance or geometry", map[string]any{"leg_id": leg.ID})
	}
	if math.IsNaN(position.Point.Lat) || math.IsNaN(position.Point.Lon) || position.Point.Lat < -90 || position.Point.Lat > 90 || position.Point.Lon < -180 || position.Point.Lon > 180 {
		return contracts.InvalidPlan("PositionAt returned invalid coordinates", map[string]any{"leg_id": leg.ID})
	}
	return nil
}

func cloneLeg(leg contracts.Leg) contracts.Leg {
	leg.Geometry = append([]contracts.Point(nil), leg.Geometry...)
	return leg
}
