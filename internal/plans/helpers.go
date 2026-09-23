package plans

import (
	"fmt"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

func cloneSnapshot(value contracts.Snapshot) contracts.Snapshot {
	result := value
	result.Locations = nonNil(append([]contracts.Location(nil), value.Locations...))
	result.Orders = nonNil(cloneOrders(value.Orders))
	result.Engineers = nonNil(cloneEngineers(value.Engineers))
	result.Issues = nonNil(append([]contracts.Issue(nil), value.Issues...))
	return result
}

func cloneOrders(values []contracts.Order) []contracts.Order {
	result := make([]contracts.Order, len(values))
	for i, value := range values {
		result[i] = value
		result[i].RequiredSkills = append([]string(nil), value.RequiredSkills...)
		result[i].EquipmentRequired = cloneEquipment(value.EquipmentRequired)
		if value.Execution != nil {
			execution := *value.Execution
			result[i].Execution = &execution
		}
	}
	return result
}

func cloneEngineers(values []contracts.Engineer) []contracts.Engineer {
	result := make([]contracts.Engineer, len(values))
	for i, value := range values {
		result[i] = value
		result[i].Skills = append([]string(nil), value.Skills...)
		result[i].EquipmentStock = cloneEquipment(value.EquipmentStock)
	}
	return result
}

func cloneEquipment(value map[contracts.Equipment]int64) map[contracts.Equipment]int64 {
	result := make(map[contracts.Equipment]int64, len(value))
	for equipment, count := range value {
		result[equipment] = count
	}
	return result
}

func cloneMatrix(value contracts.TravelMatrix) contracts.TravelMatrix {
	result := value
	result.LocationIDs = append([]string(nil), value.LocationIDs...)
	result.Profiles = make(map[contracts.Transport][][]contracts.TravelCell, len(value.Profiles))
	for profile, rows := range value.Profiles {
		copiedRows := make([][]contracts.TravelCell, len(rows))
		for i := range rows {
			copiedRows[i] = append([]contracts.TravelCell(nil), rows[i]...)
		}
		result.Profiles[profile] = copiedRows
	}
	return result
}

func cloneSolveRequest(value contracts.SolveRequest) contracts.SolveRequest {
	value.Orders = cloneOrders(value.Orders)
	value.Engineers = cloneEngineers(value.Engineers)
	value.EngineerStates = append([]contracts.EngineerState(nil), value.EngineerStates...)
	for index := range value.EngineerStates {
		value.EngineerStates[index].EquipmentAvailable = cloneEquipment(value.EngineerStates[index].EquipmentAvailable)
	}
	value.AlreadyUsedEngineerIDs = append([]string(nil), value.AlreadyUsedEngineerIDs...)
	value.TravelMatrix = cloneMatrix(value.TravelMatrix)
	return value
}

func locationMap(snapshot contracts.Snapshot) map[string]contracts.Location {
	result := make(map[string]contracts.Location, len(snapshot.Locations))
	for _, location := range snapshot.Locations {
		result[location.ID] = location
	}
	return result
}

func orderMap(orders []contracts.Order) map[string]contracts.Order {
	result := make(map[string]contracts.Order, len(orders))
	for _, order := range orders {
		result[order.ID] = order
	}
	return result
}

func engineerMap(engineers []contracts.Engineer) map[string]contracts.Engineer {
	result := make(map[string]contracts.Engineer, len(engineers))
	for _, engineer := range engineers {
		result[engineer.ID] = engineer
	}
	return result
}

func activeValidOrders(snapshot contracts.Snapshot) []contracts.Order {
	invalid := make(map[string]struct{})
	for _, issue := range snapshot.Issues {
		if issue.EntityID != nil {
			invalid[*issue.EntityID] = struct{}{}
		}
	}
	result := make([]contracts.Order, 0, len(snapshot.Orders))
	for _, order := range snapshot.Orders {
		if order.Status != contracts.OrderStatusActive && order.Status != contracts.OrderStatusSent && order.Status != contracts.OrderStatusEnRoute {
			continue
		}
		if _, exists := invalid[order.ID]; exists {
			continue
		}
		result = append(result, order)
	}
	return result
}

func availableEngineers(snapshot contracts.Snapshot) []contracts.Engineer {
	result := make([]contracts.Engineer, 0, len(snapshot.Engineers))
	for _, engineer := range snapshot.Engineers {
		if engineer.Available {
			result = append(result, engineer)
		}
	}
	return result
}

func equipmentRemaining(snapshot contracts.Snapshot) (map[string]map[contracts.Equipment]int64, error) {
	result := make(map[string]map[contracts.Equipment]int64, len(snapshot.Engineers))
	for _, engineer := range snapshot.Engineers {
		result[engineer.ID] = cloneEquipment(engineer.EquipmentStock)
	}
	for _, order := range snapshot.Orders {
		if order.Execution == nil || order.Execution.StartedAt == nil {
			continue
		}
		remaining, exists := result[order.Execution.EngineerID]
		if !exists {
			return nil, contracts.InvalidInput("started order references an unknown engineer", map[string]any{"order_id": order.ID, "engineer_id": order.Execution.EngineerID})
		}
		for equipment, count := range order.EquipmentRequired {
			remaining[equipment] -= count
			if remaining[equipment] < 0 {
				return nil, contracts.InvalidInput("started work exceeds engineer equipment stock", map[string]any{"order_id": order.ID, "engineer_id": order.Execution.EngineerID, "equipment": equipment})
			}
		}
	}
	return result, nil
}

func statusOrderIDs(snapshot contracts.Snapshot, status contracts.OrderStatus) []string {
	result := make([]string, 0)
	for _, order := range snapshot.Orders {
		if order.Status == status {
			result = append(result, order.ID)
		}
	}
	sort.Strings(result)
	return nonNil(result)
}

func profilesFor(engineers []contracts.Engineer) []contracts.Transport {
	seen := make(map[contracts.Transport]struct{})
	for _, engineer := range engineers {
		seen[engineer.Transport] = struct{}{}
	}
	result := make([]contracts.Transport, 0, len(seen))
	for profile := range seen {
		result = append(result, profile)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func relevantLocations(snapshot contracts.Snapshot, orders []contracts.Order, states []contracts.EngineerState) ([]contracts.Location, error) {
	locations := locationMap(snapshot)
	needed := map[string]struct{}{snapshot.OfficeLocationID: {}}
	for _, order := range orders {
		needed[order.LocationID] = struct{}{}
	}
	for _, state := range states {
		needed[state.StartLocationID] = struct{}{}
	}
	ids := make([]string, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]contracts.Location, 0, len(ids))
	for _, id := range ids {
		location, exists := locations[id]
		if !exists {
			return nil, contracts.InvalidInput("location referenced by planning input was not found", map[string]any{"location_id": id})
		}
		result = append(result, location)
	}
	return result, nil
}

func localDayBounds(snapshot contracts.Snapshot) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(snapshot.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, contracts.InvalidInput("invalid snapshot timezone", map[string]any{"timezone": snapshot.Timezone})
	}
	date, err := time.ParseInLocation("2006-01-02", snapshot.Date, location)
	if err != nil {
		return time.Time{}, time.Time{}, contracts.InvalidInput("invalid snapshot date", map[string]any{"date": snapshot.Date})
	}
	return date.UTC(), date.AddDate(0, 0, 1).UTC(), nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func uniqueID(prefix string, used map[string]struct{}) string {
	if _, exists := used[prefix]; !exists {
		used[prefix] = struct{}{}
		return prefix
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", prefix, index)
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func ptr[T any](value T) *T {
	return &value
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
