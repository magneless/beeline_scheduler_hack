package plans

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/contracts"
)

type replayResult struct {
	routes        []contracts.Route
	states        map[string]contracts.EngineerState
	lockedOrders  map[string]struct{}
	usedEngineers []string
	geoContextID  *string
}

func (service *Service) replayAt(ctx context.Context, snapshot *contracts.Snapshot, base contracts.Plan, event contracts.Event) (replayResult, error) {
	orders := orderMap(snapshot.Orders)
	usedLocationIDs := make(map[string]struct{}, len(snapshot.Locations))
	for _, location := range snapshot.Locations {
		usedLocationIDs[location.ID] = struct{}{}
	}
	usedLegIDs := make(map[string]struct{})
	for _, route := range base.Routes {
		for _, leg := range route.Legs {
			usedLegIDs[leg.ID] = struct{}{}
		}
	}

	result := replayResult{
		states:       make(map[string]contracts.EngineerState, len(snapshot.Engineers)),
		lockedOrders: make(map[string]struct{}),
	}
	for _, engineer := range snapshot.Engineers {
		availableFrom := event.OccurredAt
		if engineer.Shift.Start.After(availableFrom) {
			availableFrom = engineer.Shift.Start
		}
		result.states[engineer.ID] = contracts.EngineerState{
			EngineerID:      engineer.ID,
			StartLocationID: snapshot.OfficeLocationID,
			AvailableFrom:   availableFrom,
		}
	}

	seenEngineers := make(map[string]struct{})
	for _, route := range base.Routes {
		if _, exists := seenEngineers[route.EngineerID]; exists {
			return replayResult{}, contracts.InvalidPlan("base plan has multiple routes for one engineer", map[string]any{"engineer_id": route.EngineerID})
		}
		seenEngineers[route.EngineerID] = struct{}{}
		engineerState, exists := result.states[route.EngineerID]
		if !exists {
			return replayResult{}, contracts.InvalidPlan("base plan references an unknown engineer", map[string]any{"engineer_id": route.EngineerID})
		}

		preserved := contracts.Route{
			EngineerID:      route.EngineerID,
			StartLocationID: route.StartLocationID,
			StartAt:         route.StartAt,
			Visits:          []contracts.Visit{},
			Legs:            []contracts.Leg{},
		}
		var activeVisit *contracts.Visit
		var activeLeg *contracts.Leg
		var latestVisit *contracts.Visit
		var latestLeg *contracts.Leg

		for _, visit := range route.Visits {
			order, exists := orders[visit.OrderID]
			if !exists {
				return replayResult{}, contracts.InvalidPlan("base plan visit references an unknown order", map[string]any{"order_id": visit.OrderID})
			}
			if event.OccurredAt.Before(visit.StartAt) {
				continue
			}
			if event.OccurredAt.Before(visit.EndAt) {
				if activeVisit != nil {
					return replayResult{}, contracts.InvalidPlan("base plan has overlapping active visits", map[string]any{"engineer_id": route.EngineerID})
				}
				copyVisit := visit
				activeVisit = &copyVisit
				preserved.Visits = append(preserved.Visits, visit)
				result.lockedOrders[visit.OrderID] = struct{}{}
				engineerState.StartLocationID = order.LocationID
				engineerState.AvailableFrom = visit.EndAt
				continue
			}
			preserved.Visits = append(preserved.Visits, visit)
			result.lockedOrders[visit.OrderID] = struct{}{}
			copyVisit := visit
			if latestVisit == nil || copyVisit.EndAt.After(latestVisit.EndAt) {
				latestVisit = &copyVisit
			}
		}

		for _, leg := range route.Legs {
			if result.geoContextID == nil {
				result.geoContextID = ptr(leg.GeoContextID)
			} else if *result.geoContextID != leg.GeoContextID {
				return replayResult{}, contracts.InvalidPlan("base plan uses multiple geo contexts", map[string]any{"first": *result.geoContextID, "other": leg.GeoContextID})
			}
			if event.OccurredAt.Before(leg.StartAt) {
				continue
			}
			if event.OccurredAt.Before(leg.EndAt) {
				if activeLeg != nil || activeVisit != nil {
					return replayResult{}, contracts.InvalidPlan("base plan has overlapping active work or travel", map[string]any{"engineer_id": route.EngineerID})
				}
				copyLeg := leg
				activeLeg = &copyLeg
				continue
			}
			preserved.Legs = append(preserved.Legs, cloneLeg(leg))
			copyLeg := leg
			if latestLeg == nil || copyLeg.EndAt.After(latestLeg.EndAt) {
				latestLeg = &copyLeg
			}
		}

		if activeLeg != nil {
			position, err := service.geo.PositionAt(ctx, contracts.PositionRequest{Leg: cloneLeg(*activeLeg), At: event.OccurredAt})
			if err != nil {
				return replayResult{}, dependencyError("resolve position on active leg", err)
			}
			if err := validatePosition(*activeLeg, event.OccurredAt, position); err != nil {
				return replayResult{}, err
			}
			locationID := uniqueID(fmt.Sprintf("event-position-%s-%s", event.ID, route.EngineerID), usedLocationIDs)
			snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: locationID, Address: "", Point: position.Point})
			partialLegID := uniqueID(activeLeg.ID+"-elapsed-"+event.ID, usedLegIDs)
			preserved.Legs = append(preserved.Legs, contracts.Leg{
				ID:             partialLegID,
				FromLocationID: activeLeg.FromLocationID,
				ToLocationID:   locationID,
				StartAt:        activeLeg.StartAt,
				EndAt:          event.OccurredAt,
				DistanceM:      position.ElapsedDistanceM,
				GeoContextID:   activeLeg.GeoContextID,
				Geometry:       append([]contracts.Point(nil), position.ElapsedGeometry...),
			})
			engineerState.StartLocationID = locationID
			engineerState.AvailableFrom = event.OccurredAt
		} else if activeVisit == nil {
			if latestVisit != nil && (latestLeg == nil || !latestVisit.EndAt.Before(latestLeg.EndAt)) {
				engineerState.StartLocationID = orders[latestVisit.OrderID].LocationID
			} else if latestLeg != nil {
				engineerState.StartLocationID = latestLeg.ToLocationID
			}
		}

		result.states[route.EngineerID] = engineerState
		if len(preserved.Visits) > 0 || len(preserved.Legs) > 0 {
			result.routes = append(result.routes, preserved)
			result.usedEngineers = append(result.usedEngineers, route.EngineerID)
		}
	}
	sort.Strings(result.usedEngineers)
	return result, nil
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
