package plans

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
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
	engineers := engineerMap(snapshot.Engineers)
	remainingEquipment, err := equipmentRemaining(*snapshot)
	if err != nil {
		return replayResult{}, err
	}
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
			EngineerID:         engineer.ID,
			StartLocationID:    snapshot.OfficeLocationID,
			AvailableFrom:      availableFrom,
			EquipmentAvailable: cloneEquipment(remainingEquipment[engineer.ID]),
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
		for index, visit := range route.Visits {
			order, exists := orders[visit.OrderID]
			if !exists {
				return replayResult{}, contracts.InvalidPlan("base plan visit references an unknown order", map[string]any{"order_id": visit.OrderID})
			}
			execution := order.Execution
			if execution == nil {
				continue
			}
			if execution.StartedAt != nil {
				result.lockedOrders[order.ID] = struct{}{}
				if index < len(route.Legs) {
					preserved.Legs = append(preserved.Legs, cloneLeg(route.Legs[index]))
				}
				actualVisit := visit
				actualVisit.StartAt = *execution.StartedAt
				if actualVisit.ArrivalAt.After(actualVisit.StartAt) {
					actualVisit.ArrivalAt = actualVisit.StartAt
				}
				switch order.Status {
				case contracts.OrderStatusCompleted, contracts.OrderStatusCancelled:
					actualVisit.EndAt = *execution.FinishedAt
				default:
					actualVisit.EndAt = event.OccurredAt
					if execution.ExpectedEndAt != nil && execution.ExpectedEndAt.After(event.OccurredAt) {
						actualVisit.EndAt = *execution.ExpectedEndAt
					}
				}
				preserved.Visits = append(preserved.Visits, actualVisit)
				engineer := engineers[route.EngineerID]
				if actualVisit.StartAt.Before(order.Window.Start) || actualVisit.StartAt.After(order.Window.End) || actualVisit.StartAt.Before(engineer.Shift.Start) || actualVisit.EndAt.After(engineer.Shift.End) || actualVisit.EndAt.Sub(actualVisit.StartAt) != time.Duration(order.ServiceSec)*time.Second {
					appendIssueOnce(snapshot, order.ID, "ACTUAL_CONSTRAINT_VIOLATION", "Подтверждённые фактические времена нарушают плановые ограничения")
				}
				engineerState.StartLocationID = order.LocationID
				engineerState.AvailableFrom = event.OccurredAt
				if order.Status == contracts.OrderStatusInProgress {
					if execution.ExpectedEndAt == nil || !execution.ExpectedEndAt.After(event.OccurredAt) {
						delete(result.states, route.EngineerID)
						appendIssueOnce(snapshot, order.ID, "EXECUTION_STATE_REQUIRED", "Уточните ожидаемое время завершения текущей работы")
					} else {
						engineerState.AvailableFrom = *execution.ExpectedEndAt
					}
				}
				continue
			}
			if execution.DepartedAt == nil || (order.Status != contracts.OrderStatusEnRoute && order.Status != contracts.OrderStatusCancelled) || index >= len(route.Legs) {
				continue
			}
			actualLeg := cloneLeg(route.Legs[index])
			duration := actualLeg.EndAt.Sub(actualLeg.StartAt)
			actualLeg.StartAt = *execution.DepartedAt
			actualLeg.EndAt = actualLeg.StartAt.Add(duration)
			if !event.OccurredAt.After(actualLeg.StartAt) {
				engineerState.StartLocationID = actualLeg.FromLocationID
				engineerState.AvailableFrom = event.OccurredAt
				continue
			}
			if !event.OccurredAt.Before(actualLeg.EndAt) {
				preserved.Legs = append(preserved.Legs, actualLeg)
				engineerState.StartLocationID = actualLeg.ToLocationID
				engineerState.AvailableFrom = event.OccurredAt
				continue
			}
			position, err := service.geo.PositionAt(ctx, contracts.PositionRequest{Leg: actualLeg, At: event.OccurredAt})
			if err != nil {
				return replayResult{}, dependencyError("resolve position on active leg", err)
			}
			if err := validatePosition(actualLeg, event.OccurredAt, position); err != nil {
				return replayResult{}, err
			}
			locationID := uniqueID(fmt.Sprintf("event-position-%s-%s", event.ID, route.EngineerID), usedLocationIDs)
			snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: locationID, Point: position.Point})
			preserved.Legs = append(preserved.Legs, contracts.Leg{
				ID: uniqueID(actualLeg.ID+"-elapsed-"+event.ID, usedLegIDs), FromLocationID: actualLeg.FromLocationID,
				ToLocationID: locationID, StartAt: actualLeg.StartAt, EndAt: event.OccurredAt,
				DistanceM: position.ElapsedDistanceM, GeoContextID: actualLeg.GeoContextID,
				Geometry: append([]contracts.Point(nil), position.ElapsedGeometry...),
			})
			engineerState.StartLocationID = locationID
			engineerState.AvailableFrom = event.OccurredAt
		}

		if _, blocked := result.states[route.EngineerID]; !blocked {
			// The state was intentionally removed for an in-progress order
			// without a usable completion estimate.
		} else {
			result.states[route.EngineerID] = engineerState
		}
		if len(preserved.Visits) > 0 || len(preserved.Legs) > 0 {
			result.routes = append(result.routes, preserved)
			result.usedEngineers = append(result.usedEngineers, route.EngineerID)
		}
	}
	for _, route := range base.Routes {
		for _, leg := range route.Legs {
			if result.geoContextID == nil {
				result.geoContextID = ptr(leg.GeoContextID)
			} else if *result.geoContextID != leg.GeoContextID {
				return replayResult{}, contracts.InvalidPlan("base plan uses multiple geo contexts", map[string]any{"first": *result.geoContextID, "other": leg.GeoContextID})
			}
		}
	}
	sort.Strings(result.usedEngineers)
	return result, nil
}

func appendIssueOnce(snapshot *contracts.Snapshot, entityID, code, message string) {
	for _, issue := range snapshot.Issues {
		if issue.EntityID != nil && *issue.EntityID == entityID && issue.Code == code {
			return
		}
	}
	snapshot.Issues = append(snapshot.Issues, contracts.Issue{EntityID: ptr(entityID), Code: code, Message: message})
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
