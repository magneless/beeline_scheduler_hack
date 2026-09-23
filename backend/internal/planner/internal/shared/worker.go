// Package shared provides validation and route primitives for planner modes.
package shared

import (
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// Worker owns the mutable route state of one engineer within one solve call.
type Worker struct {
	Engineer        contracts.Engineer
	Skills          map[string]bool
	Remaining       map[contracts.Equipment]int64
	Location        int
	Time            time.Time
	InitialLocation int
	reachable       []bool // Optimistic connectivity from the initial location.
	Route           contracts.Route
}

func (w *Worker) MatchesSkills(order contracts.Order) bool {
	for _, skill := range order.RequiredSkills {
		if !w.Skills[skill] {
			return false
		}
	}
	return true
}

func (w *Worker) MatchesTransport(order contracts.Order) bool {
	return order.RequiredTransport == nil || *order.RequiredTransport == w.Engineer.Transport
}

func (w *Worker) HasEquipment(order contracts.Order) bool {
	for equipment, count := range order.EquipmentRequired {
		if w.Remaining[equipment] < count {
			return false
		}
	}
	return true
}

func (w *Worker) AppendCandidate(order contracts.Order, cell contracts.TravelCell) (contracts.Visit, contracts.Leg, bool) {
	if !cell.Reachable {
		return contracts.Visit{}, contracts.Leg{}, false
	}
	departure := Later(w.Time, order.ReceivedAt).UTC()
	travel := time.Duration(*cell.DurationSec) * time.Second
	if departure.After(w.Engineer.Shift.End) || travel > w.Engineer.Shift.End.Sub(departure) {
		return contracts.Visit{}, contracts.Leg{}, false
	}
	arrival := departure.Add(travel)
	start := Later(arrival, order.Window.Start).UTC()
	service := time.Duration(order.ServiceSec) * time.Second
	if start.After(order.Window.End) || start.After(w.Engineer.Shift.End) || service > w.Engineer.Shift.End.Sub(start) {
		return contracts.Visit{}, contracts.Leg{}, false
	}
	return contracts.Visit{
			OrderID: order.ID, ArrivalAt: arrival, StartAt: start, EndAt: start.Add(service),
		}, contracts.Leg{
			ToLocationID: order.LocationID, StartAt: departure, EndAt: arrival, DistanceM: *cell.DistanceM,
		}, true
}

func Later(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}

func Unassigned(order contracts.Order, reason contracts.UnassignedReason, message string) contracts.UnassignedOrder {
	return contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: reason, Message: message}
}
