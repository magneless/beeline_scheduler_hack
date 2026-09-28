package plans

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

// A non-emergency event keeps the destination of an automatically detected trip.
// The elapsed segment stays factual and the remaining segment is part of the
// protected schedule before any other work is offered to the solver.
func (service *Service) protectEnRoute(ctx context.Context, target contracts.Snapshot, base contracts.Plan, replay *replayResult, event contracts.Event) error {
	orders := orderMap(target.Orders)
	engineers := engineerMap(target.Engineers)
	for engineerID, orderID := range replay.enrouteOrders {
		if _, started := replay.lockedOrders[orderID]; !started {
			// Urgent redirection deliberately removed the lock.
			continue
		}
		order := orders[orderID]
		if order.ID == "" {
			delete(replay.lockedOrders, orderID)
			continue
		}
		if order.Status == contracts.OrderStatusCompleted || order.Status == contracts.OrderStatusInProgress || order.Status == contracts.OrderStatusCancelled && order.Execution != nil && order.Execution.StartedAt != nil {
			continue
		}
		cancelledInTransit := order.Status == contracts.OrderStatusCancelled
		if cancelledInTransit {
			delete(replay.lockedOrders, orderID)
		}
		var savedVisit contracts.Visit
		var savedLeg contracts.Leg
		found := false
		for _, route := range base.Routes {
			if route.EngineerID != engineerID {
				continue
			}
			for _, visit := range route.Visits {
				if visit.OrderID != orderID {
					continue
				}
				for _, leg := range route.Legs {
					if leg.ToLocationID == order.LocationID && leg.EndAt.Equal(visit.ArrivalAt) {
						savedVisit, savedLeg, found = visit, leg, true
					}
				}
				break
			}
		}
		if !found {
			return contracts.InvalidPlan("en-route visit is missing from the accepted plan", map[string]any{"order_id": orderID})
		}
		state := replay.states[engineerID]
		visit := savedVisit
		if visit.StartAt.Before(event.OccurredAt) {
			visit.StartAt = event.OccurredAt
			visit.EndAt = visit.StartAt.Add(time.Duration(order.ServiceSec) * time.Second)
		}
		if !cancelledInTransit && visit.EndAt.After(engineers[engineerID].Shift.End) {
			return contracts.EventConflict("current trip cannot finish within the engineer shift", map[string]any{"order_id": orderID})
		}
		var remaining *contracts.Leg
		if state.StartLocationID != order.LocationID {
			elapsedDistance := int64(0)
			for _, route := range replay.routes {
				if route.EngineerID == engineerID {
					for _, leg := range route.Legs {
						if leg.ToLocationID == state.StartLocationID {
							elapsedDistance = leg.DistanceM
						}
					}
				}
			}
			leg := contracts.Leg{ID: fmt.Sprintf("protected-%s-%s", event.ID, orderID), FromLocationID: state.StartLocationID, ToLocationID: order.LocationID, StartAt: event.OccurredAt, EndAt: savedLeg.EndAt, DistanceM: savedLeg.DistanceM - elapsedDistance, GeoContextID: savedLeg.GeoContextID}
			if leg.EndAt.Before(leg.StartAt) || leg.DistanceM < 0 {
				return contracts.InvalidPlan("invalid remaining travel on protected trip", map[string]any{"order_id": orderID})
			}
			var elapsed []contracts.Point
			for _, route := range replay.routes {
				if route.EngineerID == engineerID {
					for _, old := range route.Legs {
						if old.ToLocationID == state.StartLocationID {
							elapsed = old.Geometry
						}
					}
				}
			}
			leg.Geometry = remainingGeometry(savedLeg.Geometry, elapsed)
			remaining = &leg
			visit.ArrivalAt = leg.EndAt
		} else if visit.ArrivalAt.After(event.OccurredAt) {
			visit.ArrivalAt = event.OccurredAt
		}
		if visit.StartAt.Before(visit.ArrivalAt) {
			visit.StartAt = visit.ArrivalAt
			visit.EndAt = visit.StartAt.Add(time.Duration(order.ServiceSec) * time.Second)
		}
		index := -1
		for i := range replay.routes {
			if replay.routes[i].EngineerID == engineerID {
				index = i
				break
			}
		}
		if index < 0 {
			replay.routes = append(replay.routes, contracts.Route{EngineerID: engineerID, StartLocationID: state.StartLocationID, StartAt: state.AvailableFrom, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}})
			index = len(replay.routes) - 1
		}
		if remaining != nil {
			replay.routes[index].Legs = append(replay.routes[index].Legs, *remaining)
		}
		if !cancelledInTransit {
			replay.routes[index].Visits = append(replay.routes[index].Visits, visit)
		}
		state.StartLocationID = order.LocationID
		state.AvailableFrom = visit.EndAt
		if cancelledInTransit {
			state.AvailableFrom = savedLeg.EndAt
			if state.AvailableFrom.Before(event.OccurredAt) {
				state.AvailableFrom = event.OccurredAt
			}
		}
		if !cancelledInTransit {
			for equipment, amount := range order.EquipmentRequired {
				state.EquipmentAvailable[equipment] -= amount
			}
		}
		replay.states[engineerID] = state
	}
	return nil
}

func remainingGeometry(full, elapsed []contracts.Point) []contracts.Point {
	if len(full) == 0 {
		return nil
	}
	if len(elapsed) == 0 {
		return append([]contracts.Point(nil), full...)
	}
	current := elapsed[len(elapsed)-1]
	segment, distance := 0, math.Inf(1)
	for i := 0; i+1 < len(full); i++ {
		a, b := full[i], full[i+1]
		dx, dy := b.Lat-a.Lat, b.Lon-a.Lon
		fraction := 0.0
		if length := dx*dx + dy*dy; length > 0 {
			fraction = ((current.Lat-a.Lat)*dx + (current.Lon-a.Lon)*dy) / length
		}
		fraction = math.Max(0, math.Min(1, fraction))
		d := math.Hypot(current.Lat-(a.Lat+fraction*dx), current.Lon-(a.Lon+fraction*dy))
		if d < distance {
			segment, distance = i, d
		}
	}
	points := []contracts.Point{current}
	if segment+1 < len(full) {
		points = append(points, full[segment+1:]...)
	}
	if len(points) == 1 && current != full[len(full)-1] {
		points = append(points, full[len(full)-1])
	}
	return points
}
