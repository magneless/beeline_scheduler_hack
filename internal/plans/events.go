package plans

import (
	"context"
	"math"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

func (service *Service) applyEvent(ctx context.Context, snapshot *contracts.Snapshot, base contracts.Plan, event contracts.Event) (contracts.Event, error) {
	switch event.Type {
	case contracts.EventUrgentOrderAdded:
		return service.applyUrgentOrder(ctx, snapshot, event)
	case contracts.EventOrderCancelled:
		return applyCancellation(snapshot, event)
	case contracts.EventEngineerUnavailable:
		return applyEngineerUnavailable(snapshot, event)
	case contracts.EventOrderStatusChanged:
		return applyStatusChange(snapshot, base, event)
	default:
		return contracts.Event{}, contracts.InvalidInput("unsupported event type", map[string]any{"event_type": event.Type})
	}
}

func (service *Service) applyUrgentOrder(ctx context.Context, snapshot *contracts.Snapshot, event contracts.Event) (contracts.Event, error) {
	if event.Payload.Order == nil {
		return contracts.Event{}, contracts.InvalidInput("urgent_order_added requires payload.order", nil)
	}
	order := cloneOrders([]contracts.Order{*event.Payload.Order})[0]
	if order.ID == "" || order.LocationID == "" {
		return contracts.Event{}, contracts.InvalidInput("new emergency order id and location_id are required", nil)
	}
	for _, existing := range snapshot.Orders {
		if existing.ID == order.ID {
			return contracts.Event{}, contracts.EventConflict("emergency order id already exists", map[string]any{"order_id": order.ID})
		}
	}
	if order.Window.End.Before(order.Window.Start) || len(order.RequiredSkills) == 0 {
		return contracts.Event{}, contracts.InvalidInput("new emergency order requires a valid window and skills", map[string]any{"order_id": order.ID})
	}
	if err := validateEquipment("order equipment requirement", order.ID, order.EquipmentRequired); err != nil {
		return contracts.Event{}, err
	}
	order.WorkType = contracts.WorkTypeEmergency
	order.Priority = contracts.PriorityUrgent
	order.Status = contracts.OrderStatusActive
	order.Execution = nil
	order.ServiceSec = 4800
	order.ReceivedAt = event.OccurredAt
	var maxSourceOrder int64
	for _, existing := range snapshot.Orders {
		if existing.SourceOrder > maxSourceOrder {
			maxSourceOrder = existing.SourceOrder
		}
	}
	order.SourceOrder = maxSourceOrder + 1

	locations := locationMap(*snapshot)
	location, exists := locations[order.LocationID]
	if exists && event.Payload.Location != nil && event.Payload.Location.ID != order.LocationID {
		return contracts.Event{}, contracts.InvalidInput("payload.location id does not match emergency order", map[string]any{"location_id": order.LocationID})
	}
	if !exists {
		if event.Payload.Location == nil || event.Payload.Location.ID != order.LocationID {
			return contracts.Event{}, contracts.InvalidInput("new emergency order requires matching payload.location", map[string]any{"location_id": order.LocationID})
		}
		geocoded, err := service.geo.Geocode(ctx, contracts.GeocodeRequest{RegionID: snapshot.RegionID, Locations: []contracts.LocationInput{*event.Payload.Location}})
		if err != nil {
			return contracts.Event{}, dependencyError("geocode emergency order", err)
		}
		if len(geocoded.Items) != 1 || geocoded.Items[0].LocationID != order.LocationID || geocoded.Items[0].Location == nil || geocoded.Items[0].Issue != nil {
			return contracts.Event{}, contracts.EventConflict("emergency order address could not be resolved", map[string]any{"location_id": order.LocationID})
		}
		location = *geocoded.Items[0].Location
		if location.ID != order.LocationID || math.IsNaN(location.Point.Lat) || math.IsNaN(location.Point.Lon) || location.Point.Lat < -90 || location.Point.Lat > 90 || location.Point.Lon < -180 || location.Point.Lon > 180 {
			return contracts.Event{}, contracts.InvalidInput("geocoder returned an invalid location", map[string]any{"location_id": order.LocationID})
		}
		snapshot.Locations = append(snapshot.Locations, location)
	}
	snapshot.Orders = append(snapshot.Orders, order)
	normalized := event
	normalized.Payload = contracts.EventPayload{Order: &order, Location: &contracts.LocationInput{ID: location.ID, Address: location.Address, Point: ptr(location.Point)}}
	return normalized, nil
}

func applyCancellation(snapshot *contracts.Snapshot, event contracts.Event) (contracts.Event, error) {
	if event.Payload.OrderID == "" || (event.Payload.Reason != contracts.CancellationClientRefusal && event.Payload.Reason != contracts.CancellationCannotPerform) {
		return contracts.Event{}, contracts.InvalidInput("order_cancelled requires order_id and a supported reason", nil)
	}
	for index := range snapshot.Orders {
		order := &snapshot.Orders[index]
		if order.ID != event.Payload.OrderID {
			continue
		}
		if order.Status == contracts.OrderStatusCompleted || order.Status == contracts.OrderStatusCancelled {
			return contracts.Event{}, contracts.EventConflict("completed or cancelled order cannot be cancelled", map[string]any{"order_id": order.ID})
		}
		order.Status = contracts.OrderStatusCancelled
		if order.Execution != nil {
			order.Execution.ExpectedEndAt = nil
			if order.Execution.StartedAt != nil {
				finished := event.OccurredAt
				if !finished.After(*order.Execution.StartedAt) {
					return contracts.Event{}, contracts.EventConflict("cancellation must be after work start", map[string]any{"order_id": order.ID})
				}
				order.Execution.FinishedAt = &finished
			}
		}
		normalized := event
		normalized.Payload = contracts.EventPayload{OrderID: order.ID, Reason: event.Payload.Reason}
		return normalized, nil
	}
	return contracts.Event{}, contracts.EventConflict("order was not found", map[string]any{"order_id": event.Payload.OrderID})
}

func applyEngineerUnavailable(snapshot *contracts.Snapshot, event contracts.Event) (contracts.Event, error) {
	if event.Payload.EngineerID == "" {
		return contracts.Event{}, contracts.InvalidInput("engineer_unavailable requires payload.engineer_id", nil)
	}
	for index := range snapshot.Engineers {
		if snapshot.Engineers[index].ID != event.Payload.EngineerID {
			continue
		}
		if !snapshot.Engineers[index].Available {
			return contracts.Event{}, contracts.EventConflict("engineer is already unavailable", map[string]any{"engineer_id": event.Payload.EngineerID})
		}
		snapshot.Engineers[index].Available = false
		normalized := event
		normalized.Payload = contracts.EventPayload{EngineerID: event.Payload.EngineerID}
		return normalized, nil
	}
	return contracts.Event{}, contracts.EventConflict("engineer was not found", map[string]any{"engineer_id": event.Payload.EngineerID})
}

func applyStatusChange(snapshot *contracts.Snapshot, base contracts.Plan, event contracts.Event) (contracts.Event, error) {
	payload := event.Payload
	if payload.OrderID == "" || payload.EngineerID == "" {
		return contracts.Event{}, contracts.InvalidInput("order_status_changed requires order_id and engineer_id", nil)
	}
	if payload.Status != contracts.OrderStatusInProgress && payload.ExpectedEndAt != nil {
		return contracts.Event{}, contracts.InvalidInput("expected_end_at is allowed only for in_progress", map[string]any{"order_id": payload.OrderID})
	}
	if payload.ExpectedEndAt != nil && !payload.ExpectedEndAt.After(event.OccurredAt) {
		return contracts.Event{}, contracts.EventConflict("expected_end_at must be after the event", map[string]any{"order_id": payload.OrderID})
	}
	assignment, assigned := assignments(base.Routes)[payload.OrderID]
	for index := range snapshot.Orders {
		order := &snapshot.Orders[index]
		if order.ID != payload.OrderID {
			continue
		}
		if order.Status == contracts.OrderStatusCompleted || order.Status == contracts.OrderStatusCancelled {
			return contracts.Event{}, contracts.EventConflict("closed order status cannot be changed", map[string]any{"order_id": order.ID})
		}
		if order.Execution == nil {
			if !assigned || assignment.EngineerID != payload.EngineerID {
				return contracts.Event{}, contracts.EventConflict("engineer does not match current plan assignment", map[string]any{"order_id": order.ID, "engineer_id": payload.EngineerID})
			}
		} else if order.Execution.EngineerID != payload.EngineerID {
			return contracts.Event{}, contracts.EventConflict("engineer does not match confirmed execution", map[string]any{"order_id": order.ID, "engineer_id": payload.EngineerID})
		}
		if err := ensureEngineerHasNoOtherCurrentWork(*snapshot, order.ID, payload.EngineerID, payload.Status); err != nil {
			return contracts.Event{}, err
		}
		switch {
		case order.Status == contracts.OrderStatusActive && payload.Status == contracts.OrderStatusSent:
			order.Status = contracts.OrderStatusSent
			order.Execution = &contracts.OrderExecution{EngineerID: payload.EngineerID}
		case order.Status == contracts.OrderStatusSent && payload.Status == contracts.OrderStatusEnRoute:
			departed := event.OccurredAt
			order.Status = contracts.OrderStatusEnRoute
			order.Execution.DepartedAt = &departed
		case (order.Status == contracts.OrderStatusSent || order.Status == contracts.OrderStatusEnRoute) && payload.Status == contracts.OrderStatusInProgress:
			if event.OccurredAt.Before(order.ReceivedAt) || event.OccurredAt.Before(order.Window.Start) {
				return contracts.Event{}, contracts.EventConflict("work cannot start before receipt or client window", map[string]any{"order_id": order.ID})
			}
			arrival := assignment.ArrivalAt
			if order.Execution.DepartedAt != nil {
				if duration, ok := plannedTravelDuration(base.Routes, order.ID); ok {
					arrival = order.Execution.DepartedAt.Add(duration)
				}
			}
			if event.OccurredAt.Before(arrival) {
				return contracts.Event{}, contracts.EventConflict("work cannot start before confirmed arrival", map[string]any{"order_id": order.ID, "arrival_at": arrival})
			}
			remaining, err := equipmentRemaining(*snapshot)
			if err != nil {
				return contracts.Event{}, err
			}
			for equipment, count := range order.EquipmentRequired {
				if remaining[payload.EngineerID][equipment] < count {
					return contracts.Event{}, contracts.EventConflict("not enough equipment to start work", map[string]any{"order_id": order.ID, "equipment": equipment})
				}
			}
			started := event.OccurredAt
			order.Status = contracts.OrderStatusInProgress
			order.Execution.StartedAt = &started
			order.Execution.ExpectedEndAt = cloneTime(payload.ExpectedEndAt)
		case order.Status == contracts.OrderStatusInProgress && payload.Status == contracts.OrderStatusInProgress:
			order.Execution.ExpectedEndAt = cloneTime(payload.ExpectedEndAt)
		case order.Status == contracts.OrderStatusInProgress && payload.Status == contracts.OrderStatusCompleted:
			if order.Execution.StartedAt == nil || !event.OccurredAt.After(*order.Execution.StartedAt) {
				return contracts.Event{}, contracts.EventConflict("completion must be after work start", map[string]any{"order_id": order.ID})
			}
			finished := event.OccurredAt
			order.Status = contracts.OrderStatusCompleted
			order.Execution.FinishedAt = &finished
			order.Execution.ExpectedEndAt = nil
		default:
			return contracts.Event{}, contracts.EventConflict("unsupported order status transition", map[string]any{"order_id": order.ID, "from": order.Status, "to": payload.Status})
		}
		normalized := event
		normalized.Payload = contracts.EventPayload{OrderID: order.ID, Status: payload.Status, EngineerID: payload.EngineerID, ExpectedEndAt: cloneTime(payload.ExpectedEndAt)}
		return normalized, nil
	}
	return contracts.Event{}, contracts.EventConflict("order was not found", map[string]any{"order_id": payload.OrderID})
}

func plannedTravelDuration(routes []contracts.Route, orderID string) (time.Duration, bool) {
	for _, route := range routes {
		for index, visit := range route.Visits {
			if visit.OrderID == orderID && index < len(route.Legs) {
				return route.Legs[index].EndAt.Sub(route.Legs[index].StartAt), true
			}
		}
	}
	return 0, false
}

func ensureEngineerHasNoOtherCurrentWork(snapshot contracts.Snapshot, orderID, engineerID string, target contracts.OrderStatus) error {
	if target != contracts.OrderStatusEnRoute && target != contracts.OrderStatusInProgress {
		return nil
	}
	for _, order := range snapshot.Orders {
		if order.ID == orderID || order.Execution == nil || order.Execution.EngineerID != engineerID {
			continue
		}
		if order.Status == contracts.OrderStatusEnRoute || order.Status == contracts.OrderStatusInProgress {
			return contracts.EventConflict("engineer already has another current execution", map[string]any{"engineer_id": engineerID, "order_id": order.ID})
		}
	}
	return nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
