package plans

import (
	"fmt"
	"math"
	"time"

	"github.com/magneless/beeline_scheduler_hack/contracts"
)

func validateSnapshot(snapshot contracts.Snapshot, scenarioID string, revision int64) error {
	if snapshot.ScenarioID == "" || snapshot.ScenarioID != scenarioID {
		return contracts.InvalidInput("snapshot scenario does not match request", map[string]any{"expected": scenarioID, "actual": snapshot.ScenarioID})
	}
	if snapshot.Revision != revision {
		return contracts.InvalidInput("snapshot revision does not match request", map[string]any{"expected": revision, "actual": snapshot.Revision})
	}
	if snapshot.RegionID == "" || snapshot.OfficeLocationID == "" {
		return contracts.InvalidInput("snapshot region and office are required", nil)
	}
	if _, _, err := localDayBounds(snapshot); err != nil {
		return err
	}
	locations := make(map[string]struct{}, len(snapshot.Locations))
	for _, location := range snapshot.Locations {
		if location.ID == "" {
			return contracts.InvalidInput("location id is required", nil)
		}
		if _, exists := locations[location.ID]; exists {
			return contracts.InvalidInput("duplicate location id", map[string]any{"location_id": location.ID})
		}
		if math.IsNaN(location.Point.Lat) || math.IsNaN(location.Point.Lon) || location.Point.Lat < -90 || location.Point.Lat > 90 || location.Point.Lon < -180 || location.Point.Lon > 180 {
			return contracts.InvalidInput("invalid location coordinates", map[string]any{"location_id": location.ID})
		}
		locations[location.ID] = struct{}{}
	}
	if _, exists := locations[snapshot.OfficeLocationID]; !exists {
		return contracts.InvalidInput("office location was not found", map[string]any{"location_id": snapshot.OfficeLocationID})
	}
	orders := make(map[string]struct{}, len(snapshot.Orders))
	for _, order := range snapshot.Orders {
		if order.ID == "" || order.LocationID == "" {
			return contracts.InvalidInput("order id and location are required", nil)
		}
		if _, exists := orders[order.ID]; exists {
			return contracts.InvalidInput("duplicate order id", map[string]any{"order_id": order.ID})
		}
		if _, exists := locations[order.LocationID]; !exists {
			return contracts.InvalidInput("order location was not found", map[string]any{"order_id": order.ID, "location_id": order.LocationID})
		}
		if order.ServiceSec <= 0 || order.Window.End.Before(order.Window.Start) {
			return contracts.InvalidInput("invalid order duration or window", map[string]any{"order_id": order.ID})
		}
		if order.Priority != contracts.PriorityNormal && order.Priority != contracts.PriorityUrgent {
			return contracts.InvalidInput("invalid order priority", map[string]any{"order_id": order.ID, "priority": order.Priority})
		}
		if order.Status != contracts.OrderStatusActive && order.Status != contracts.OrderStatusCancelled && order.Status != contracts.OrderStatusSent && order.Status != contracts.OrderStatusEnRoute && order.Status != contracts.OrderStatusInProgress && order.Status != contracts.OrderStatusCompleted {
			return contracts.InvalidInput("invalid order status", map[string]any{"order_id": order.ID, "status": order.Status})
		}
		orders[order.ID] = struct{}{}
		if order.Status == contracts.OrderStatusActive && order.Execution != nil {
			return contracts.InvalidInput("active order has execution", nil)
		}
		if order.Status != contracts.OrderStatusActive && order.Status != contracts.OrderStatusCancelled && order.Execution == nil {
			return contracts.InvalidInput("execution is required for status", nil)
		}
	}
	engineers := make(map[string]struct{}, len(snapshot.Engineers))
	for _, engineer := range snapshot.Engineers {
		if engineer.ID == "" {
			return contracts.InvalidInput("engineer id is required", nil)
		}
		if _, exists := engineers[engineer.ID]; exists {
			return contracts.InvalidInput("duplicate engineer id", map[string]any{"engineer_id": engineer.ID})
		}
		if engineer.Transport != contracts.TransportCar && engineer.Transport != contracts.TransportWalk {
			return contracts.InvalidInput("invalid engineer transport", map[string]any{"engineer_id": engineer.ID, "transport": engineer.Transport})
		}
		if engineer.Shift.End.Before(engineer.Shift.Start) {
			return contracts.InvalidInput("invalid engineer shift", map[string]any{"engineer_id": engineer.ID})
		}
		engineers[engineer.ID] = struct{}{}
	}
	return nil
}

func validateMatrix(matrix contracts.TravelMatrix, locations []contracts.Location, profiles []contracts.Transport) error {
	if matrix.ID == "" || matrix.GeoContextID == "" {
		return contracts.InvalidInput("travel matrix id and geo context are required", nil)
	}
	if len(matrix.LocationIDs) != len(locations) {
		return contracts.InvalidInput("travel matrix location count mismatch", map[string]any{"expected": len(locations), "actual": len(matrix.LocationIDs)})
	}
	expected := make(map[string]struct{}, len(locations))
	for _, location := range locations {
		expected[location.ID] = struct{}{}
	}
	for _, id := range matrix.LocationIDs {
		if _, exists := expected[id]; !exists {
			return contracts.InvalidInput("travel matrix contains an unknown location", map[string]any{"location_id": id})
		}
		delete(expected, id)
	}
	if len(expected) > 0 {
		return contracts.InvalidInput("travel matrix is missing locations", map[string]any{"missing_count": len(expected)})
	}
	size := len(matrix.LocationIDs)
	for _, profile := range profiles {
		rows, exists := matrix.Profiles[profile]
		if !exists || len(rows) != size {
			return contracts.InvalidInput("travel matrix profile is missing or has invalid size", map[string]any{"profile": profile})
		}
		for rowIndex, row := range rows {
			if len(row) != size {
				return contracts.InvalidInput("travel matrix row has invalid size", map[string]any{"profile": profile, "row": rowIndex})
			}
			for columnIndex, cell := range row {
				if cell.Reachable {
					if cell.DurationSec == nil || cell.DistanceM == nil || *cell.DurationSec < 0 || *cell.DistanceM < 0 {
						return contracts.InvalidInput("reachable matrix cell must contain non-negative duration and distance", map[string]any{"profile": profile, "row": rowIndex, "column": columnIndex})
					}
				} else if cell.DurationSec != nil || cell.DistanceM != nil {
					return contracts.InvalidInput("unreachable matrix cell must have null duration and distance", map[string]any{"profile": profile, "row": rowIndex, "column": columnIndex})
				}
				if rowIndex == columnIndex && (!cell.Reachable || cell.DurationSec == nil || cell.DistanceM == nil || *cell.DurationSec != 0 || *cell.DistanceM != 0) {
					return contracts.InvalidInput("travel matrix diagonal must be reachable with zero duration and distance", map[string]any{"profile": profile, "row": rowIndex})
				}
			}
		}
	}
	return nil
}

func validateSolveResult(request contracts.SolveRequest, result contracts.SolveResult) error {
	if result.Termination != contracts.TerminationCompleted && result.Termination != contracts.TerminationTimeLimit {
		return contracts.InvalidPlan("planner returned an invalid termination value", map[string]any{"termination": result.Termination})
	}
	orders := orderMap(request.Orders)
	engineers := engineerMap(request.Engineers)
	states := make(map[string]contracts.EngineerState, len(request.EngineerStates))
	for _, state := range request.EngineerStates {
		if _, exists := states[state.EngineerID]; exists {
			return contracts.InvalidInput("duplicate engineer state", map[string]any{"engineer_id": state.EngineerID})
		}
		states[state.EngineerID] = state
	}
	for engineerID := range engineers {
		if _, exists := states[engineerID]; !exists {
			return contracts.InvalidInput("engineer state is missing", map[string]any{"engineer_id": engineerID})
		}
	}
	accounted := make(map[string]string, len(orders))
	seenRoutes := make(map[string]struct{}, len(result.Routes))
	seenLegs := make(map[string]struct{})
	for _, route := range result.Routes {
		engineer, exists := engineers[route.EngineerID]
		if !exists {
			return contracts.InvalidPlan("route references an unknown engineer", map[string]any{"engineer_id": route.EngineerID})
		}
		if _, exists := seenRoutes[route.EngineerID]; exists {
			return contracts.InvalidPlan("planner returned multiple routes for one engineer", map[string]any{"engineer_id": route.EngineerID})
		}
		seenRoutes[route.EngineerID] = struct{}{}
		state := states[route.EngineerID]
		if route.StartLocationID != state.StartLocationID || route.StartAt.Before(state.AvailableFrom) {
			return contracts.InvalidPlan("route does not start from the supplied engineer state", map[string]any{"engineer_id": route.EngineerID})
		}
		if len(route.Visits) == 0 || len(route.Legs) != len(route.Visits) {
			return contracts.InvalidPlan("route must contain one leg per visit", map[string]any{"engineer_id": route.EngineerID})
		}
		cursorLocation := route.StartLocationID
		cursorTime := state.AvailableFrom
		stock := cloneEquipment(state.EquipmentAvailable)
		for index, visit := range route.Visits {
			order, exists := orders[visit.OrderID]
			if !exists {
				return contracts.InvalidPlan("visit references an unknown order", map[string]any{"order_id": visit.OrderID})
			}
			if previous, exists := accounted[visit.OrderID]; exists {
				return contracts.InvalidPlan("order appears more than once in planner result", map[string]any{"order_id": visit.OrderID, "first": previous})
			}
			if !hasAllSkills(engineer.Skills, order.RequiredSkills) {
				return contracts.InvalidPlan("engineer does not have required skills", map[string]any{"order_id": order.ID, "engineer_id": engineer.ID})
			}
			if order.RequiredTransport != nil && *order.RequiredTransport != engineer.Transport {
				return contracts.InvalidPlan("engineer transport does not match order", map[string]any{"order_id": order.ID, "engineer_id": engineer.ID})
			}
			leg := route.Legs[index]
			for kind, amount := range order.EquipmentRequired {
				stock[kind] -= amount
				if stock[kind] < 0 {
					return contracts.InvalidPlan("planner exceeded equipment balance", nil)
				}
			}
			if leg.ID == "" {
				return contracts.InvalidPlan("leg id is required", map[string]any{"engineer_id": engineer.ID, "sequence": index})
			}
			if _, exists := seenLegs[leg.ID]; exists {
				return contracts.InvalidPlan("duplicate leg id", map[string]any{"leg_id": leg.ID})
			}
			seenLegs[leg.ID] = struct{}{}
			if leg.FromLocationID != cursorLocation || leg.ToLocationID != order.LocationID {
				return contracts.InvalidPlan("leg endpoints do not match route sequence", map[string]any{"leg_id": leg.ID})
			}
			if leg.StartAt.Before(cursorTime) || leg.StartAt.Before(order.ReceivedAt) || visit.StartAt.Before(order.ReceivedAt) || !leg.EndAt.Equal(visit.ArrivalAt) || leg.EndAt.Before(leg.StartAt) {
				return contracts.InvalidPlan("leg times do not match visit arrival", map[string]any{"leg_id": leg.ID})
			}
			if len(leg.Geometry) != 0 {
				return contracts.InvalidPlan("planner must not populate leg geometry", map[string]any{"leg_id": leg.ID})
			}
			cell, err := matrixCell(request.TravelMatrix, engineer.Transport, leg.FromLocationID, leg.ToLocationID)
			if err != nil {
				return err
			}
			if !cell.Reachable || cell.DurationSec == nil || cell.DistanceM == nil {
				return contracts.InvalidPlan("route uses an unreachable matrix pair", map[string]any{"leg_id": leg.ID})
			}
			if leg.EndAt.Sub(leg.StartAt) != time.Duration(*cell.DurationSec)*time.Second || leg.DistanceM != *cell.DistanceM || leg.GeoContextID != request.TravelMatrix.GeoContextID {
				return contracts.InvalidPlan("leg does not match travel matrix", map[string]any{"leg_id": leg.ID})
			}
			if visit.ArrivalAt.After(visit.StartAt) || visit.StartAt.Before(order.Window.Start) || visit.StartAt.After(order.Window.End) {
				return contracts.InvalidPlan("visit does not satisfy the order window", map[string]any{"order_id": order.ID})
			}
			if visit.EndAt.Sub(visit.StartAt) != time.Duration(order.ServiceSec)*time.Second || visit.StartAt.Before(engineer.Shift.Start) || visit.EndAt.After(engineer.Shift.End) {
				return contracts.InvalidPlan("visit does not satisfy service duration or engineer shift", map[string]any{"order_id": order.ID, "engineer_id": engineer.ID})
			}
			accounted[visit.OrderID] = "route"
			cursorLocation = order.LocationID
			cursorTime = visit.EndAt
		}
	}
	for _, item := range result.Unassigned {
		if _, exists := orders[item.OrderID]; !exists {
			return contracts.InvalidPlan("unassigned item references an unknown order", map[string]any{"order_id": item.OrderID})
		}
		if previous, exists := accounted[item.OrderID]; exists {
			return contracts.InvalidPlan("order appears more than once in planner result", map[string]any{"order_id": item.OrderID, "first": previous})
		}
		if item.ReasonCode == "" || item.Message == "" {
			return contracts.InvalidPlan("unassigned order must contain reason code and message", map[string]any{"order_id": item.OrderID})
		}
		accounted[item.OrderID] = "unassigned"
	}
	if len(accounted) != len(orders) {
		for id := range orders {
			if _, exists := accounted[id]; !exists {
				return contracts.InvalidPlan("planner omitted an order", map[string]any{"order_id": id})
			}
		}
	}
	return nil
}

func hasAllSkills(engineerSkills, required []string) bool {
	for _, skill := range required {
		if !containsString(engineerSkills, skill) {
			return false
		}
	}
	return true
}

func matrixCell(matrix contracts.TravelMatrix, profile contracts.Transport, from, to string) (contracts.TravelCell, error) {
	fromIndex, toIndex := -1, -1
	for index, id := range matrix.LocationIDs {
		if id == from {
			fromIndex = index
		}
		if id == to {
			toIndex = index
		}
	}
	if fromIndex < 0 || toIndex < 0 {
		return contracts.TravelCell{}, contracts.InvalidPlan("leg references a location missing from travel matrix", map[string]any{"from": from, "to": to})
	}
	rows, exists := matrix.Profiles[profile]
	if !exists || fromIndex >= len(rows) || toIndex >= len(rows[fromIndex]) {
		return contracts.TravelCell{}, contracts.InvalidPlan("travel matrix profile is missing", map[string]any{"profile": profile})
	}
	return rows[fromIndex][toIndex], nil
}

func attachGeometry(routes []contracts.Route, response contracts.RoutesGeometry) ([]contracts.Route, error) {
	expected := make(map[string]struct{})
	for _, route := range routes {
		for _, leg := range route.Legs {
			expected[leg.ID] = struct{}{}
		}
	}
	geometries := make(map[string][]contracts.Point, len(response.Items))
	for _, item := range response.Items {
		if _, exists := expected[item.LegID]; !exists {
			return nil, contracts.InvalidPlan("geometry response contains an unknown leg", map[string]any{"leg_id": item.LegID})
		}
		if _, exists := geometries[item.LegID]; exists || len(item.Geometry) == 0 {
			return nil, contracts.InvalidPlan("geometry response contains duplicate or empty geometry", map[string]any{"leg_id": item.LegID})
		}
		geometries[item.LegID] = append([]contracts.Point(nil), item.Geometry...)
	}
	if len(geometries) != len(expected) {
		return nil, contracts.InvalidPlan("geometry response is incomplete", map[string]any{"expected": len(expected), "actual": len(geometries)})
	}
	result := make([]contracts.Route, len(routes))
	for routeIndex, route := range routes {
		result[routeIndex] = route
		result[routeIndex].Visits = append([]contracts.Visit(nil), route.Visits...)
		result[routeIndex].Legs = append([]contracts.Leg(nil), route.Legs...)
		for legIndex := range result[routeIndex].Legs {
			legID := result[routeIndex].Legs[legIndex].ID
			result[routeIndex].Legs[legIndex].Geometry = geometries[legID]
		}
	}
	return result, nil
}

func routeGeometryRequest(routes []contracts.Route, engineers map[string]contracts.Engineer, locations []contracts.Location, geoContextID string) (contracts.RoutesRequest, error) {
	request := contracts.RoutesRequest{GeoContextID: geoContextID, Locations: append([]contracts.Location(nil), locations...)}
	for _, route := range routes {
		engineer, exists := engineers[route.EngineerID]
		if !exists {
			return contracts.RoutesRequest{}, contracts.InvalidPlan("route references an unknown engineer", map[string]any{"engineer_id": route.EngineerID})
		}
		for _, leg := range route.Legs {
			request.Legs = append(request.Legs, contracts.RouteLegRequest{LegID: leg.ID, FromLocationID: leg.FromLocationID, ToLocationID: leg.ToLocationID, Profile: engineer.Transport})
		}
	}
	return request, nil
}

func dependencyError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
