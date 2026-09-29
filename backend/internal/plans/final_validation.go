package plans

import (
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func validateFinalPlan(snapshot contracts.Snapshot, routes []contracts.Route, unassigned []contracts.UnassignedOrder, cancelledIDs []string, lateness ...[]contracts.OrderLateness) error {
	orders := orderMap(snapshot.Orders)
	late := map[string]contracts.OrderLateness{}
	if len(lateness) > 0 {
		for _, item := range lateness[0] {
			order, ok := orders[item.OrderID]
			if !ok || order.WorkType != contracts.WorkTypeEmergency || order.Priority != contracts.PriorityUrgent || item.Window != order.Window || item.LateSec <= 0 || item.StartAt.Sub(order.Window.End) != time.Duration(item.LateSec)*time.Second {
				return contracts.InvalidPlan("invalid emergency lateness", map[string]any{"order_id": item.OrderID})
			}
			if _, exists := late[item.OrderID]; exists {
				return contracts.InvalidPlan("duplicate emergency lateness", nil)
			}
			late[item.OrderID] = item
		}
	}
	engineers := engineerMap(snapshot.Engineers)
	accounted := make(map[string]string)
	seenRoutes := make(map[string]struct{})
	seenLegs := make(map[string]struct{})
	for _, route := range routes {
		engineer, exists := engineers[route.EngineerID]
		if !exists {
			return contracts.InvalidPlan("final route references an unknown engineer", map[string]any{"engineer_id": route.EngineerID})
		}
		if _, exists := seenRoutes[route.EngineerID]; exists {
			return contracts.InvalidPlan("final plan has multiple routes for one engineer", map[string]any{"engineer_id": route.EngineerID})
		}
		seenRoutes[route.EngineerID] = struct{}{}
		type interval struct {
			start time.Time
			end   time.Time
			kind  string
			id    string
		}
		intervals := make([]interval, 0, len(route.Visits)+len(route.Legs))
		for sequence, visit := range route.Visits {
			order, exists := orders[visit.OrderID]
			if !exists || (order.Status == contracts.OrderStatusCancelled && (order.Execution == nil || order.Execution.StartedAt == nil)) {
				return contracts.InvalidPlan("final visit references a missing or inactive order", map[string]any{"order_id": visit.OrderID})
			}
			if previous, exists := accounted[visit.OrderID]; exists {
				return contracts.InvalidPlan("order appears more than once in final plan", map[string]any{"order_id": visit.OrderID, "first": previous})
			}
			actual := order.Execution != nil && order.Execution.StartedAt != nil
			if !actual && (!hasAllSkills(engineer.Skills, order.RequiredSkills) || order.RequiredTransport != nil && *order.RequiredTransport != engineer.Transport) {
				return contracts.InvalidPlan("final assignment violates skill or transport requirements", map[string]any{"order_id": order.ID, "engineer_id": engineer.ID})
			}
			lateItem, allowedLate := late[order.ID]
			if visit.ArrivalAt.After(visit.StartAt) || visit.EndAt.Before(visit.StartAt) || (!actual && (visit.StartAt.Before(order.Window.Start) || (visit.StartAt.After(order.Window.End) && !allowedLate) || visit.EndAt.Sub(visit.StartAt) != time.Duration(order.ServiceSec)*time.Second)) {
				return contracts.InvalidPlan("final visit violates order timing", map[string]any{"order_id": order.ID})
			}
			if allowedLate {
				if !actual && (!visit.StartAt.After(order.Window.End) || !visit.ArrivalAt.Equal(lateItem.ArrivalAt) || !visit.StartAt.Equal(lateItem.StartAt)) {
					return contracts.InvalidPlan("emergency lateness does not match visit", map[string]any{"order_id": order.ID})
				}
				delete(late, order.ID)
			}
			if !actual && (visit.StartAt.Before(engineer.Shift.Start) || visit.EndAt.After(engineer.Shift.End)) {
				return contracts.InvalidPlan("final visit is outside engineer shift", map[string]any{"order_id": order.ID, "engineer_id": engineer.ID})
			}
			accounted[visit.OrderID] = "route"
			intervals = append(intervals, interval{start: visit.StartAt, end: visit.EndAt, kind: "visit", id: visit.OrderID})
			_ = sequence
		}
		for _, leg := range route.Legs {
			if leg.ID == "" || leg.GeoContextID == "" || leg.DistanceM < 0 || leg.EndAt.Before(leg.StartAt) || len(leg.Geometry) == 0 {
				return contracts.InvalidPlan("final leg has invalid fields", map[string]any{"leg_id": leg.ID})
			}
			if _, exists := seenLegs[leg.ID]; exists {
				return contracts.InvalidPlan("final plan has duplicate leg id", map[string]any{"leg_id": leg.ID})
			}
			seenLegs[leg.ID] = struct{}{}
			// A zero-distance transition at the same address occupies no time.
			if !leg.StartAt.Equal(leg.EndAt) || leg.DistanceM != 0 {
				intervals = append(intervals, interval{start: leg.StartAt, end: leg.EndAt, kind: "leg", id: leg.ID})
			}
		}
		sort.SliceStable(intervals, func(i, j int) bool {
			if intervals[i].start.Equal(intervals[j].start) {
				return intervals[i].end.Before(intervals[j].end)
			}
			return intervals[i].start.Before(intervals[j].start)
		})
		for index := 1; index < len(intervals); index++ {
			if intervals[index].start.Before(intervals[index-1].end) {
				return contracts.InvalidPlan("final route contains overlapping travel or work", map[string]any{"engineer_id": route.EngineerID, "first": intervals[index-1].id, "second": intervals[index].id})
			}
		}
	}
	for _, item := range unassigned {
		order, exists := orders[item.OrderID]
		if !exists || (order.Status != contracts.OrderStatusActive && order.Status != contracts.OrderStatusSent && order.Status != contracts.OrderStatusEnRoute) {
			return contracts.InvalidPlan("unassigned item references a missing or inactive order", map[string]any{"order_id": item.OrderID})
		}
		if previous, exists := accounted[item.OrderID]; exists {
			return contracts.InvalidPlan("order appears more than once in final plan", map[string]any{"order_id": item.OrderID, "first": previous})
		}
		if item.ReasonCode == "" || item.Message == "" {
			return contracts.InvalidPlan("unassigned order must contain reason code and message", map[string]any{"order_id": item.OrderID})
		}
		accounted[item.OrderID] = "unassigned"
	}
	if len(late) > 0 {
		return contracts.InvalidPlan("lateness references an unassigned order", nil)
	}
	cancelled := make(map[string]struct{}, len(cancelledIDs))
	for _, orderID := range cancelledIDs {
		if _, exists := cancelled[orderID]; exists {
			return contracts.InvalidPlan("cancelled_order_ids contains a duplicate", map[string]any{"order_id": orderID})
		}
		order, exists := orders[orderID]
		if !exists || order.Status != contracts.OrderStatusCancelled {
			return contracts.InvalidPlan("cancelled_order_ids references a missing or active order", map[string]any{"order_id": orderID})
		}
		cancelled[orderID] = struct{}{}
	}
	for _, order := range activeValidOrders(snapshot) {
		if _, exists := accounted[order.ID]; !exists {
			return contracts.InvalidPlan("active order is missing from final plan", map[string]any{"order_id": order.ID})
		}
	}
	return nil
}

func mergeRoutes(preserved, future []contracts.Route) []contracts.Route {
	result := make([]contracts.Route, 0, len(preserved)+len(future))
	indexByEngineer := make(map[string]int)
	for _, route := range preserved {
		copyRoute := route
		copyRoute.Visits = nonNil(append([]contracts.Visit(nil), route.Visits...))
		copyRoute.Legs = make([]contracts.Leg, len(route.Legs))
		for index, leg := range route.Legs {
			copyRoute.Legs[index] = cloneLeg(leg)
		}
		indexByEngineer[route.EngineerID] = len(result)
		result = append(result, copyRoute)
	}
	for _, route := range future {
		if index, exists := indexByEngineer[route.EngineerID]; exists {
			result[index].Visits = append(result[index].Visits, route.Visits...)
			for _, leg := range route.Legs {
				result[index].Legs = append(result[index].Legs, cloneLeg(leg))
			}
			continue
		}
		copyRoute := route
		copyRoute.Visits = nonNil(append([]contracts.Visit(nil), route.Visits...))
		copyRoute.Legs = make([]contracts.Leg, len(route.Legs))
		for index, leg := range route.Legs {
			copyRoute.Legs[index] = cloneLeg(leg)
		}
		indexByEngineer[route.EngineerID] = len(result)
		result = append(result, copyRoute)
	}
	return nonNil(result)
}

func renameCollidingFutureLegs(routes []contracts.Route, preserved []contracts.Route, eventID string) []contracts.Route {
	used := make(map[string]struct{})
	for _, route := range preserved {
		for _, leg := range route.Legs {
			used[leg.ID] = struct{}{}
		}
	}
	result := make([]contracts.Route, len(routes))
	for routeIndex, route := range routes {
		result[routeIndex] = route
		result[routeIndex].Visits = append([]contracts.Visit(nil), route.Visits...)
		result[routeIndex].Legs = append([]contracts.Leg(nil), route.Legs...)
		for legIndex := range result[routeIndex].Legs {
			leg := &result[routeIndex].Legs[legIndex]
			if _, exists := used[leg.ID]; exists {
				leg.ID = uniqueID("future-"+eventID+"-"+leg.ID, used)
			} else {
				used[leg.ID] = struct{}{}
			}
		}
	}
	return result
}
