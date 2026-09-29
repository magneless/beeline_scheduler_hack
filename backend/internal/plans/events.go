package plans

import (
	"context"
	"encoding/json"
	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
	"math"
	"time"
)

func (service *Service) applyEvent(ctx context.Context, snapshot *contracts.Snapshot, base contracts.Plan, event contracts.Event, replay replayResult) (contracts.Event, []string, error) {
	var payload contracts.EventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return contracts.Event{}, nil, contracts.InvalidInput("invalid event payload", nil)
	}
	normalized := event
	switch event.Type {
	case contracts.EventUrgentOrderAdded, contracts.EventOrdinaryOrderAdded:
		if payload.Order == nil {
			return normalized, nil, contracts.InvalidInput("new order event requires order", nil)
		}
		order := cloneOrders([]contracts.Order{*payload.Order})[0]
		var restored *contracts.Order
		for _, item := range snapshot.UnlocatedOrders {
			if item.Order.ID == order.ID {
				original := cloneOrders([]contracts.Order{item.Order})[0]
				original.ReceivedAt = event.OccurredAt
				order = original
				restored = &original
				break
			}
		}
		if order.ID == "" || order.LocationID == "" || order.Status != contracts.OrderStatusActive || order.Execution != nil || order.ServiceSec <= 0 || !order.ReceivedAt.Equal(event.OccurredAt) || order.Window.End.Before(order.Window.Start) {
			return normalized, nil, contracts.InvalidInput("invalid new order", nil)
		}
		if event.Type == contracts.EventUrgentOrderAdded {
			if order.WorkType != contracts.WorkTypeEmergency || order.Priority != contracts.PriorityUrgent || order.ServiceSec != 4800 {
				return normalized, nil, contracts.InvalidInput("invalid new order", nil)
			}
		} else if order.Priority != contracts.PriorityNormal || (order.WorkType != contracts.WorkTypeConnection && order.WorkType != contracts.WorkTypeRepair && order.WorkType != contracts.WorkTypeAdditional) {
			return normalized, nil, contracts.InvalidInput("invalid ordinary order", nil)
		}
		order.SourceOrder = 1
		for _, existing := range snapshot.Orders {
			if existing.ID == order.ID {
				return normalized, nil, contracts.EventConflict("order already exists", nil)
			}
			if existing.SourceOrder >= order.SourceOrder {
				order.SourceOrder = existing.SourceOrder + 1
			}
		}
		if restored != nil {
			order.SourceOrder = restored.SourceOrder
		}
		loc, exists := locationMap(*snapshot)[order.LocationID]
		if payload.Location != nil && payload.Location.ID != order.LocationID {
			return normalized, nil, contracts.InvalidInput("location id mismatch", nil)
		}
		if !exists {
			if payload.Location == nil {
				return normalized, nil, contracts.InvalidInput("new location is required", nil)
			}
			progress.Report(ctx, "geocoding", "Проверяем адрес новой заявки")
			out, err := service.geo.Geocode(ctx, contracts.GeocodeRequest{RegionID: snapshot.RegionID, Locations: []contracts.LocationInput{*payload.Location}})
			if err != nil {
				return normalized, nil, dependencyError("geocode new order", err)
			}
			if len(out.Items) == 1 && out.Items[0].LocationID == order.LocationID && out.Items[0].Issue != nil {
				issue := out.Items[0].Issue
				return normalized, nil, contracts.EventConflict(issue.Message, map[string]any{"address": payload.Location.Address, "geocode_issue": issue.Code})
			}
			if len(out.Items) != 1 || out.Items[0].LocationID != order.LocationID || out.Items[0].Location == nil || out.Items[0].Issue != nil {
				return normalized, nil, contracts.EventConflict("Адрес не найден или неоднозначен. Уточните город, улицу и номер дома.", nil)
			}
			loc = *out.Items[0].Location
			if loc.ID != order.LocationID || math.IsNaN(loc.Point.Lat) || math.IsNaN(loc.Point.Lon) || loc.Point.Lat < -90 || loc.Point.Lat > 90 || loc.Point.Lon < -180 || loc.Point.Lon > 180 {
				return normalized, nil, contracts.InvalidInput("invalid new address coordinates", nil)
			}
			snapshot.Locations = append(snapshot.Locations, loc)
		}
		snapshot.Orders = append(snapshot.Orders, order)
		// A confirmed address restores the same imported order, including its
		// work type, priority and equipment; it must not remain in two lists.
		unlocated := make([]contracts.UnlocatedOrder, 0, len(snapshot.UnlocatedOrders))
		for _, item := range snapshot.UnlocatedOrders {
			if item.Order.ID != order.ID {
				unlocated = append(unlocated, item)
			}
		}
		snapshot.UnlocatedOrders = unlocated
		issues := make([]contracts.Issue, 0, len(snapshot.Issues))
		for _, issue := range snapshot.Issues {
			if issue.EntityID == nil || *issue.EntityID != order.LocationID {
				issues = append(issues, issue)
			}
		}
		snapshot.Issues = issues
		payload.Order = &order
		payload.Location = &contracts.LocationInput{ID: loc.ID, Address: loc.Address, Point: ptr(loc.Point)}
		normalized.Payload = contracts.EncodePayload(payload)
		return normalized, nil, nil
	case contracts.EventEngineerUnavailable:
		for n := range snapshot.Engineers {
			eng := &snapshot.Engineers[n]
			if eng.ID == payload.EngineerID {
				if !eng.Available && !eng.Reserve {
					return normalized, nil, contracts.EventConflict("engineer already unavailable", nil)
				}
				eng.Available = false
				eng.Reserve = false
				return normalized, nil, nil
			}
		}
		return normalized, nil, contracts.EventConflict("engineer not found", nil)
	case contracts.EventOrderCancelled, contracts.EventOrderStatusChanged:
		var order *contracts.Order
		for n := range snapshot.Orders {
			if snapshot.Orders[n].ID == payload.OrderID {
				order = &snapshot.Orders[n]
				break
			}
		}
		if order == nil {
			return normalized, nil, contracts.EventConflict("order not found", nil)
		}
		if order.Status == contracts.OrderStatusCompleted || order.Status == contracts.OrderStatusCancelled {
			return normalized, nil, contracts.EventConflict("closed order cannot change", nil)
		}
		if event.Type == contracts.EventOrderCancelled {
			if payload.Reason != "client_refusal" && payload.Reason != "cannot_perform" {
				return normalized, nil, contracts.InvalidInput("cancellation reason is required", nil)
			}
			if order.Execution != nil && order.Execution.StartedAt != nil && order.Execution.FinishedAt == nil {
				return normalized, nil, contracts.EventConflict("started work cannot be interrupted by cancellation", nil)
			}
			order.Status = contracts.OrderStatusCancelled
			return normalized, []string{order.ID}, nil
		}
		engineer, exists := engineerMap(snapshot.Engineers)[payload.EngineerID]
		if !exists {
			return normalized, nil, contracts.EventConflict("engineer not found", nil)
		}
		assigned := false
		for _, route := range base.Routes {
			for _, visit := range route.Visits {
				if visit.OrderID == order.ID && route.EngineerID == engineer.ID {
					assigned = true
				}
			}
		}
		if !assigned || (order.Execution != nil && order.Execution.EngineerID != engineer.ID) {
			return normalized, nil, contracts.EventConflict("status does not match current assignment", nil)
		}
		if payload.ExpectedEndAt != nil && (payload.Status != contracts.OrderStatusInProgress || !payload.ExpectedEndAt.After(event.OccurredAt)) {
			return normalized, nil, contracts.InvalidInput("invalid expected_end_at", nil)
		}
		switch payload.Status {
		case contracts.OrderStatusSent:
			if order.Status != contracts.OrderStatusActive {
				return normalized, nil, contracts.EventConflict("only active order can be sent", nil)
			}
			order.Execution = &contracts.OrderExecution{EngineerID: engineer.ID}
		case contracts.OrderStatusEnRoute:
			if order.Status != contracts.OrderStatusSent {
				return normalized, nil, contracts.EventConflict("only sent order can depart", nil)
			}
			if !engineer.Available || event.OccurredAt.Before(order.ReceivedAt) || event.OccurredAt.Before(engineer.Shift.Start) {
				return normalized, nil, contracts.EventConflict("departure precedes availability", nil)
			}
			if err := ensureIdle(*snapshot, order.ID, engineer.ID); err != nil {
				return normalized, nil, err
			}
			order.Execution.DepartedAt = ptr(event.OccurredAt)
		case contracts.OrderStatusInProgress:
			if !engineer.Available {
				return normalized, nil, contracts.EventConflict("Инженер недоступен для начала работы", nil)
			}
			if order.Status == contracts.OrderStatusInProgress {
				order.Execution.ExpectedEndAt = payload.ExpectedEndAt
				return normalized, nil, nil
			}
			if order.Status != contracts.OrderStatusActive && order.Status != contracts.OrderStatusSent && order.Status != contracts.OrderStatusEnRoute {
				return normalized, nil, contracts.EventConflict("order must be assigned before work starts", nil)
			}
			visit, found := assignedVisit(base, order.ID, engineer.ID)
			if !found || event.OccurredAt.Before(visit.ArrivalAt) || event.OccurredAt.Before(order.Window.Start) || event.OccurredAt.Before(order.ReceivedAt) || event.OccurredAt.Before(engineer.Shift.Start) {
				return normalized, nil, contracts.EventConflict("work cannot start before arrival or window", nil)
			}
			if err := validateWorkFact(*snapshot, order.ID, engineer.ID, event.OccurredAt, nil); err != nil {
				return normalized, nil, err
			}
			stock, err := equipmentRemaining(*snapshot)
			if err != nil {
				return normalized, nil, err
			}
			for kind, amount := range order.EquipmentRequired {
				if stock[engineer.ID][kind] < amount {
					return normalized, nil, contracts.EventConflict("insufficient equipment", nil)
				}
			}
			if order.Execution == nil {
				order.Execution = &contracts.OrderExecution{EngineerID: engineer.ID}
			}
			if order.Execution.DepartedAt == nil {
				for _, route := range base.Routes {
					if route.EngineerID != engineer.ID {
						continue
					}
					for _, leg := range route.Legs {
						if leg.ToLocationID == order.LocationID && leg.EndAt.Equal(visit.ArrivalAt) {
							order.Execution.DepartedAt = ptr(leg.StartAt)
						}
					}
				}
			}
			order.Execution.StartedAt = ptr(event.OccurredAt)
			order.Execution.ExpectedEndAt = payload.ExpectedEndAt
		case contracts.OrderStatusCompleted:
			if order.Status != contracts.OrderStatusInProgress || order.Execution.StartedAt == nil || !event.OccurredAt.After(*order.Execution.StartedAt) {
				return normalized, nil, contracts.EventConflict("only started work can be completed", nil)
			}
			if err := validateWorkFact(*snapshot, order.ID, engineer.ID, *order.Execution.StartedAt, &event.OccurredAt); err != nil {
				return normalized, nil, err
			}
			order.Execution.FinishedAt = ptr(event.OccurredAt)
			order.Execution.ExpectedEndAt = nil
		default:
			return normalized, nil, contracts.InvalidInput("unsupported status", nil)
		}
		order.Status = payload.Status
		return normalized, nil, nil
	default:
		return normalized, nil, contracts.InvalidInput("unsupported event", nil)
	}
}
func ensureIdle(s contracts.Snapshot, orderID, engineerID string) error {
	for _, o := range s.Orders {
		if o.ID != orderID && o.Execution != nil && o.Execution.EngineerID == engineerID && (o.Status == contracts.OrderStatusInProgress || o.Status == contracts.OrderStatusEnRoute) {
			return contracts.EventConflict("engineer has another current trip or work", nil)
		}
	}
	return nil
}

func assignedVisit(base contracts.Plan, orderID, engineerID string) (contracts.Visit, bool) {
	for _, route := range base.Routes {
		if route.EngineerID != engineerID {
			continue
		}
		for _, visit := range route.Visits {
			if visit.OrderID == orderID {
				return visit, true
			}
		}
	}
	return contracts.Visit{}, false
}

// Missing reports are not evidence that earlier work is still ongoing. Check
// known facts only; never infer a completion or spend equipment twice.
func validateWorkFact(snapshot contracts.Snapshot, orderID, engineerID string, start time.Time, end *time.Time) error {
	for _, other := range snapshot.Orders {
		ex := other.Execution
		if other.ID == orderID || ex == nil || ex.EngineerID != engineerID || ex.StartedAt == nil {
			continue
		}
		conflict := start.Equal(*ex.StartedAt)
		if ex.FinishedAt != nil {
			conflict = conflict || (!start.Before(*ex.StartedAt) && start.Before(*ex.FinishedAt))
		}
		if end != nil && start.Before(*ex.StartedAt) && end.After(*ex.StartedAt) {
			conflict = true
		}
		if conflict {
			return contracts.EventConflict("Фактическое время пересекается с другой работой этой бригады", map[string]any{"order_id": orderID, "other_order_id": other.ID})
		}
	}
	return nil
}

// Facts may arrive in either order. Replanning needs a finish for older work
// once a later actual start is known; a forecast is not a reported completion.
func requireHistoricalCompletions(snapshot contracts.Snapshot) error {
	for _, order := range snapshot.Orders {
		ex := order.Execution
		if ex == nil || ex.StartedAt == nil || ex.FinishedAt != nil {
			continue
		}
		for _, other := range snapshot.Orders {
			next := other.Execution
			if next != nil && next.EngineerID == ex.EngineerID && next.StartedAt != nil && next.StartedAt.After(*ex.StartedAt) {
				return contracts.EventConflict("Перед пересчётом укажите фактическое завершение предыдущей работы", map[string]any{"order_id": order.ID, "address": locationMap(snapshot)[order.LocationID].Address})
			}
		}
	}
	return nil
}
