package testutil

import (
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

var TestDay = time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)

func At(h, m int) time.Time {
	return TestDay.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}
func Ptr(v int64) *int64 { return &v }

func BaseRequest() contracts.SolveRequest {
	return contracts.SolveRequest{
		Mode:           contracts.SolveModeBaseline,
		Orders:         []contracts.Order{Order("o1", "p1", 1, 7, 0, 10, 0)},
		Engineers:      []contracts.Engineer{Engineer("e1", 1)},
		EngineerStates: []contracts.EngineerState{{EngineerID: "e1", StartLocationID: "depot", AvailableFrom: At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}}},
		TravelMatrix:   Matrix([]string{"depot", "p1"}, contracts.TransportCar),
		TimeLimitMS:    1000,
	}
}

func Order(id, loc string, source int64, wh, wm, service, received int) contracts.Order {
	return contracts.Order{ID: id, LocationID: loc, WorkType: contracts.WorkTypeRepair, RequiredSkills: []string{"repair"}, Window: contracts.Window{Start: At(wh, wm), End: At(17, 0)}, ReceivedAt: At(received, 0), ServiceSec: int64(service * 60), Priority: contracts.PriorityNormal, EquipmentRequired: map[contracts.Equipment]int64{}, SourceOrder: source, Status: contracts.OrderStatusActive}
}

func Engineer(id string, source int64) contracts.Engineer {
	return contracts.Engineer{ID: id, Skills: []string{"repair"}, Transport: contracts.TransportCar, Shift: contracts.Window{Start: At(6, 0), End: At(18, 0)}, Available: true, EquipmentStock: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}, SourceOrder: source}
}

func Matrix(locations []string, profile contracts.Transport) contracts.TravelMatrix {
	n := len(locations)
	cells := make([][]contracts.TravelCell, n)
	for i := range cells {
		cells[i] = make([]contracts.TravelCell, n)
		for j := range cells[i] {
			sec, dist := int64(0), int64(0)
			if i != j {
				sec, dist = 600, 1000
			}
			cells[i][j] = contracts.TravelCell{Reachable: true, DurationSec: &sec, DistanceM: &dist}
		}
	}
	return contracts.TravelMatrix{ID: "m1", GeoContextID: "geo1", LocationIDs: locations, Profiles: map[contracts.Transport][][]contracts.TravelCell{profile: cells}}
}

func AssignedIDs(got contracts.SolveResult) map[string]string {
	out := map[string]string{}
	for _, route := range got.Routes {
		for _, visit := range route.Visits {
			out[visit.OrderID] = route.EngineerID
		}
	}
	return out
}

func TransportPtr(v contracts.Transport) *contracts.Transport { return &v }
func TravelCell(duration, distance int64) contracts.TravelCell {
	return contracts.TravelCell{Reachable: true, DurationSec: Ptr(duration), DistanceM: Ptr(distance)}
}
func UnreachableCell() contracts.TravelCell { return contracts.TravelCell{} }
