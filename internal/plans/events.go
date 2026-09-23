package plans

import (
	"context"
	"math"

	"github.com/magneless/beeline_scheduler_hack/contracts"
)

func (service *Service) applyEvent(ctx context.Context, snapshot *contracts.Snapshot, event contracts.Event, lockedOrders map[string]struct{}) (contracts.Event, []string, error) {
	normalized := event
	switch event.Type {
	case contracts.EventUrgentOrderAdded:
		if event.Payload.Order == nil {
			return contracts.Event{}, nil, contracts.InvalidInput("urgent_order_added requires payload.order", nil)
		}
		order := *event.Payload.Order
		order.RequiredSkills = append([]string(nil), event.Payload.Order.RequiredSkills...)
		if order.ID == "" || order.LocationID == "" {
			return contracts.Event{}, nil, contracts.InvalidInput("new urgent order id and location_id are required", nil)
		}
		for _, existing := range snapshot.Orders {
			if existing.ID == order.ID {
				return contracts.Event{}, nil, contracts.EventConflict("urgent order id already exists", map[string]any{"order_id": order.ID})
			}
		}
		if order.ServiceSec <= 0 || order.Window.End.Before(order.Window.Start) {
			return contracts.Event{}, nil, contracts.InvalidInput("new urgent order has invalid duration or window", map[string]any{"order_id": order.ID})
		}
		order.Priority = contracts.PriorityUrgent
		order.Status = contracts.OrderStatusActive
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
			return contracts.Event{}, nil, contracts.InvalidInput("payload.location id does not match urgent order", map[string]any{"location_id": order.LocationID})
		}
		if !exists {
			if event.Payload.Location == nil || event.Payload.Location.ID != order.LocationID {
				return contracts.Event{}, nil, contracts.InvalidInput("new urgent order requires matching payload.location", map[string]any{"location_id": order.LocationID})
			}
			geocoded, err := service.geo.Geocode(ctx, contracts.GeocodeRequest{RegionID: snapshot.RegionID, Locations: []contracts.LocationInput{*event.Payload.Location}})
			if err != nil {
				return contracts.Event{}, nil, dependencyError("geocode urgent order", err)
			}
			if len(geocoded.Items) != 1 || geocoded.Items[0].LocationID != order.LocationID || geocoded.Items[0].Location == nil || geocoded.Items[0].Issue != nil {
				return contracts.Event{}, nil, contracts.EventConflict("urgent order address could not be resolved", map[string]any{"location_id": order.LocationID})
			}
			location = *geocoded.Items[0].Location
			if location.ID != order.LocationID || math.IsNaN(location.Point.Lat) || math.IsNaN(location.Point.Lon) || location.Point.Lat < -90 || location.Point.Lat > 90 || location.Point.Lon < -180 || location.Point.Lon > 180 {
				return contracts.Event{}, nil, contracts.InvalidInput("geocoder returned an invalid location", map[string]any{"location_id": order.LocationID})
			}
			snapshot.Locations = append(snapshot.Locations, location)
		}
		snapshot.Orders = append(snapshot.Orders, order)
		normalized.Payload.Order = &order
		normalized.Payload.Location = &contracts.LocationInput{ID: location.ID, Address: location.Address, Point: ptr(location.Point)}
		normalized.Payload.OrderID = ""
		normalized.Payload.EngineerID = ""
		return normalized, nil, nil

	case contracts.EventOrderCancelled:
		if event.Payload.OrderID == "" {
			return contracts.Event{}, nil, contracts.InvalidInput("order_cancelled requires payload.order_id", nil)
		}
		if _, locked := lockedOrders[event.Payload.OrderID]; locked {
			return contracts.Event{}, nil, contracts.EventConflict("started or completed order cannot be cancelled", map[string]any{"order_id": event.Payload.OrderID})
		}
		found := false
		for index := range snapshot.Orders {
			if snapshot.Orders[index].ID != event.Payload.OrderID {
				continue
			}
			found = true
			if snapshot.Orders[index].Status != contracts.OrderStatusActive {
				return contracts.Event{}, nil, contracts.EventConflict("order is not active", map[string]any{"order_id": event.Payload.OrderID})
			}
			snapshot.Orders[index].Status = contracts.OrderStatusCancelled
			break
		}
		if !found {
			return contracts.Event{}, nil, contracts.EventConflict("order was not found", map[string]any{"order_id": event.Payload.OrderID})
		}
		normalized.Payload = contracts.EventPayload{OrderID: event.Payload.OrderID}
		return normalized, []string{event.Payload.OrderID}, nil

	case contracts.EventEngineerUnavailable:
		if event.Payload.EngineerID == "" {
			return contracts.Event{}, nil, contracts.InvalidInput("engineer_unavailable requires payload.engineer_id", nil)
		}
		found := false
		for index := range snapshot.Engineers {
			if snapshot.Engineers[index].ID != event.Payload.EngineerID {
				continue
			}
			found = true
			if !snapshot.Engineers[index].Available {
				return contracts.Event{}, nil, contracts.EventConflict("engineer is already unavailable", map[string]any{"engineer_id": event.Payload.EngineerID})
			}
			snapshot.Engineers[index].Available = false
			break
		}
		if !found {
			return contracts.Event{}, nil, contracts.EventConflict("engineer was not found", map[string]any{"engineer_id": event.Payload.EngineerID})
		}
		normalized.Payload = contracts.EventPayload{EngineerID: event.Payload.EngineerID}
		return normalized, nil, nil
	default:
		return contracts.Event{}, nil, contracts.InvalidInput("unsupported event type", map[string]any{"event_type": event.Type})
	}
}
