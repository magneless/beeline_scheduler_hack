package planner_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/internal/planner"
)

var testDay = time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)

func at(h, m int) time.Time {
	return testDay.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}
func ptr(v int64) *int64 { return &v }

func baseRequest() contracts.SolveRequest {
	return contracts.SolveRequest{
		Mode:           contracts.SolveModeBaseline,
		Orders:         []contracts.Order{order("o1", "p1", 1, 7, 0, 10, 0)},
		Engineers:      []contracts.Engineer{engineer("e1", 1)},
		EngineerStates: []contracts.EngineerState{{EngineerID: "e1", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}}},
		TravelMatrix:   matrix([]string{"depot", "p1"}, contracts.TransportCar),
		TimeLimitMS:    1000,
	}
}

func order(id, loc string, source int64, wh, wm, service, received int) contracts.Order {
	return contracts.Order{ID: id, LocationID: loc, WorkType: contracts.WorkTypeRepair, RequiredSkills: []string{"repair"}, Window: contracts.Window{Start: at(wh, wm), End: at(17, 0)}, ReceivedAt: at(received, 0), ServiceSec: int64(service * 60), Priority: contracts.PriorityNormal, EquipmentRequired: map[contracts.Equipment]int64{}, SourceOrder: source, Status: contracts.OrderStatusActive}
}

func engineer(id string, source int64) contracts.Engineer {
	return contracts.Engineer{ID: id, Skills: []string{"repair"}, Transport: contracts.TransportCar, Shift: contracts.Window{Start: at(6, 0), End: at(18, 0)}, Available: true, EquipmentStock: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}, SourceOrder: source}
}

func matrix(locations []string, profile contracts.Transport) contracts.TravelMatrix {
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

func solve(t *testing.T, in contracts.SolveRequest) (contracts.SolveResult, error) {
	t.Helper()
	return planner.NewBaseline().Solve(context.Background(), in)
}

func requireInvalid(t *testing.T, in contracts.SolveRequest) {
	t.Helper()
	_, err := solve(t, in)
	var ce *contracts.ContractError
	if !errors.As(err, &ce) || ce.Code != "INVALID_INPUT" {
		t.Fatalf("Solve() error = %v, want ContractError INVALID_INPUT", err)
	}
}

func assignedIDs(got contracts.SolveResult) map[string]string {
	out := map[string]string{}
	for _, route := range got.Routes {
		for _, visit := range route.Visits {
			out[visit.OrderID] = route.EngineerID
		}
	}
	return out
}

func TestBaselineBackendFlowExample(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "contracts", "examples", "backend_flow.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var example struct {
		SolveRequest contracts.SolveRequest `json:"solve_request"`
	}
	if err := json.Unmarshal(b, &example); err != nil {
		t.Fatal(err)
	}
	in := example.SolveRequest
	in.Mode = contracts.SolveModeBaseline
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Termination != contracts.TerminationCompleted || len(got.Routes) != 1 || len(got.Unassigned) != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	r := got.Routes[0]
	if r.EngineerID != "eng-1" || r.StartLocationID != "office-1" || !r.StartAt.Equal(at(6, 0)) || len(r.Visits) != 1 || len(r.Legs) != 1 {
		t.Fatalf("unexpected route: %+v", r)
	}
	v := r.Visits[0]
	if v.OrderID != "order-1" || !v.ArrivalAt.Equal(at(6, 15)) || !v.StartAt.Equal(at(7, 0)) || !v.EndAt.Equal(at(7, 30)) {
		t.Fatalf("unexpected visit: %+v", v)
	}
	l := r.Legs[0]
	if l.ID != "leg-1" || l.FromLocationID != "office-1" || l.ToLocationID != "loc-1" || !l.StartAt.Equal(at(6, 0)) || !l.EndAt.Equal(at(6, 15)) || l.DistanceM != 1200 || l.GeoContextID != "geo-1" {
		t.Fatalf("unexpected leg: %+v", l)
	}
}

func TestBaselineOrdersAndEngineersUseSourceOrderThenID(t *testing.T) {
	in := baseRequest()
	in.Orders = []contracts.Order{order("z", "p1", 1, 7, 0, 10, 0), order("b", "p2", 1, 7, 0, 10, 0), order("a", "p3", 1, 7, 0, 10, 0)}
	in.TravelMatrix = matrix([]string{"depot", "p1", "p2", "p3"}, contracts.TransportCar)
	in.Engineers = []contracts.Engineer{engineer("z-eng", 1), engineer("a-eng", 1)}
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "z-eng", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
		{EngineerID: "a-eng", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
	}
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "a-eng", "b": "a-eng", "z": "a-eng"}
	if actual := assignedIDs(got); !reflect.DeepEqual(actual, want) {
		t.Fatalf("assignments = %#v, want %#v", actual, want)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 3 {
		t.Fatalf("baseline should use first sorted engineer and append all visits: %+v", got.Routes)
	}
	for i, id := range []string{"a", "b", "z"} {
		if got.Routes[0].Visits[i].OrderID != id {
			t.Fatalf("visit order = %+v", got.Routes[0].Visits)
		}
	}
}

func TestBaselineDoesNotPrioritizeUrgentOrders(t *testing.T) {
	in := baseRequest()
	normal := order("normal", "p1", 1, 7, 0, 10, 0)
	urgent := order("urgent", "p2", 2, 7, 0, 10, 0)
	urgent.WorkType, urgent.Priority, urgent.ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
	in.Orders = []contracts.Order{urgent, normal}
	in.TravelMatrix = matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 2 || got.Routes[0].Visits[0].OrderID != "normal" || got.Routes[0].Visits[1].OrderID != "urgent" {
		t.Fatalf("baseline should follow source order despite priority: %+v", got.Routes)
	}
}

func TestBaselineAllowsWindowEndStartAndFinishAfterWindow(t *testing.T) {
	in := baseRequest()
	in.Orders[0].Window = contracts.Window{Start: at(7, 10), End: at(7, 10)}
	in.Orders[0].ServiceSec = 30 * 60
	in.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = ptr(600)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	v := got.Routes[0].Visits[0]
	if !v.StartAt.Equal(at(7, 10)) || !v.EndAt.Equal(at(7, 40)) {
		t.Fatalf("visit = %+v; start at window end and finish after it should be allowed", v)
	}
}

func TestBaselineRespectsReceivedAtForDeparture(t *testing.T) {
	in := baseRequest()
	in.Orders[0].ReceivedAt = at(8, 0)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !got.Routes[0].Legs[0].StartAt.Equal(at(8, 0)) || !got.Routes[0].Visits[0].ArrivalAt.Equal(at(8, 10)) {
		t.Fatalf("received_at must constrain departure to the order: route=%+v", got.Routes[0])
	}
}

func TestBaselineUsesEngineerStateForReplanning(t *testing.T) {
	in := baseRequest()
	in.EngineerStates[0].StartLocationID = "p1"
	in.EngineerStates[0].AvailableFrom = at(8, 0)
	in.Orders[0].LocationID = "p2"
	in.TravelMatrix = matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].StartLocationID != "p1" || !got.Routes[0].StartAt.Equal(at(8, 0)) || got.Routes[0].Legs[0].FromLocationID != "p1" {
		t.Fatalf("route did not start from live engineer state: %+v", got.Routes)
	}
}

func TestBaselineUsesDirectedProfileTravel(t *testing.T) {
	in := baseRequest()
	in.EngineerStates[0].StartLocationID = "p1"
	in.Orders[0].LocationID = "depot"
	in.TravelMatrix = matrix([]string{"depot", "p1"}, contracts.TransportCar)
	in.TravelMatrix.Profiles[contracts.TransportCar][1][0] = contracts.TravelCell{Reachable: true, DurationSec: ptr(120), DistanceM: ptr(250)}
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Legs) != 1 || got.Routes[0].Legs[0].FromLocationID != "p1" || got.Routes[0].Legs[0].DistanceM != 250 || !got.Routes[0].Visits[0].ArrivalAt.Equal(at(6, 2)) {
		t.Fatalf("expected p1→depot directed matrix leg: %+v", got.Routes)
	}
}

func TestBaselineReservesEquipmentFromEngineerStateWithoutMutatingInput(t *testing.T) {
	in := baseRequest()
	in.Orders = []contracts.Order{order("o1", "p1", 1, 7, 0, 10, 0), order("o2", "p2", 2, 7, 0, 10, 0)}
	in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders[1].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Engineers[0].EquipmentStock = map[contracts.Equipment]int64{contracts.EquipmentRouter: 99}
	in.EngineerStates[0].EquipmentAvailable = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.TravelMatrix = matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	before, _ := json.Marshal(in)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(in)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Solve mutated input")
	}
	if len(assignedIDs(got)) != 1 || len(got.Unassigned) != 1 {
		t.Fatalf("must reserve equipment_available across future visits: %+v", got)
	}
}

func TestBaselineReportsProvableStaticIneligibility(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*contracts.SolveRequest)
		want   contracts.UnassignedReason
	}{
		{"skill", func(r *contracts.SolveRequest) { r.Engineers[0].Skills = []string{"other"} }, contracts.ReasonNoMatchingSkill},
		{"transport", func(r *contracts.SolveRequest) { tr := contracts.TransportWalk; r.Orders[0].RequiredTransport = &tr }, contracts.ReasonNoMatchingTransport},
		{"equipment", func(r *contracts.SolveRequest) {
			r.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}
			r.EngineerStates[0].EquipmentAvailable = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
		}, contracts.ReasonNoMatchingEquipment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseRequest()
			tc.mutate(&in)
			got, err := solve(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Unassigned) != 1 || got.Unassigned[0].ReasonCode != tc.want {
				t.Fatalf("unassigned = %+v, want reason %s", got.Unassigned, tc.want)
			}
		})
	}
}

func TestBaselineCanReassignSentAndEnRouteOrders(t *testing.T) {
	for _, status := range []contracts.OrderStatus{contracts.OrderStatusSent, contracts.OrderStatusEnRoute} {
		t.Run(string(status), func(t *testing.T) {
			in := baseRequest()
			in.Engineers = append(in.Engineers, engineer("e2", 2))
			in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "e2", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}})
			in.Orders[0].Status = status
			execution := &contracts.OrderExecution{EngineerID: "e2"}
			if status == contracts.OrderStatusEnRoute {
				departed := at(6, 0)
				execution.DepartedAt = &departed
			}
			in.Orders[0].Execution = execution
			got, err := solve(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if actual := assignedIDs(got)[in.Orders[0].ID]; actual != "e1" {
				t.Fatalf("prior execution engineer should not bind: assigned to %q", actual)
			}
		})
	}
}

func TestBaselineRejectsInvalidInputs(t *testing.T) {
	tests := map[string]func(*contracts.SolveRequest){
		"optimized mode":       func(r *contracts.SolveRequest) { r.Mode = contracts.SolveModeOptimized },
		"unavailable engineer": func(r *contracts.SolveRequest) { r.Engineers[0].Available = false },
		"missing matrix point": func(r *contracts.SolveRequest) {
			r.TravelMatrix.LocationIDs = []string{"depot"}
			r.TravelMatrix.Profiles[contracts.TransportCar] = [][]contracts.TravelCell{{r.TravelMatrix.Profiles[contracts.TransportCar][0][0]}}
		},
		"missing profile": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles = map[contracts.Transport][][]contracts.TravelCell{}
		},
		"wrong matrix shape": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar] = r.TravelMatrix.Profiles[contracts.TransportCar][:1]
		},
		"null reachable cell": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = nil
		},
		"unreachable cell has values": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar][0][1] = contracts.TravelCell{Reachable: false, DurationSec: ptr(0), DistanceM: ptr(0)}
		},
		"negative travel": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = ptr(-1)
		},
		"bad diagonal": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar][0][0].DistanceM = ptr(1)
		},
		"duplicate order id": func(r *contracts.SolveRequest) { r.Orders = append(r.Orders, r.Orders[0]) },
		"duplicate engineer id": func(r *contracts.SolveRequest) {
			r.Engineers = append(r.Engineers, r.Engineers[0])
			r.EngineerStates = append(r.EngineerStates, r.EngineerStates[0])
		},
		"duplicate state":       func(r *contracts.SolveRequest) { r.EngineerStates = append(r.EngineerStates, r.EngineerStates[0]) },
		"invalid work priority": func(r *contracts.SolveRequest) { r.Orders[0].Priority = contracts.PriorityUrgent },
		"invalid status":        func(r *contracts.SolveRequest) { r.Orders[0].Status = contracts.OrderStatusInProgress },
		"zero service":          func(r *contracts.SolveRequest) { r.Orders[0].ServiceSec = 0 },
		"negative equipment": func(r *contracts.SolveRequest) {
			r.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = -1
		},
		"bad window": func(r *contracts.SolveRequest) { r.Orders[0].Window.End = r.Orders[0].Window.Start.Add(-time.Second) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) { in := baseRequest(); mutate(&in); requireInvalid(t, in) })
	}
}

func TestBaselineReturnsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := planner.NewBaseline().Solve(ctx, baseRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Solve() error = %v, want context.Canceled", err)
	}
}

func TestBaselineFailedAppendDoesNotConsumeTimeOrEquipment(t *testing.T) {
	in := baseRequest()
	blocked := order("too-late", "p1", 1, 7, 0, 20, 0)
	blocked.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	blocked.Window = contracts.Window{Start: at(17, 55), End: at(17, 55)}
	laterOrder := order("fits", "p2", 2, 7, 0, 20, 0)
	laterOrder.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders = []contracts.Order{blocked, laterOrder}
	in.TravelMatrix = matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	in.Engineers[0].Shift.End = at(18, 0)
	in.Engineers[0].EquipmentStock[contracts.EquipmentRouter] = 1
	in.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 1

	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if actual := assignedIDs(got); !reflect.DeepEqual(actual, map[string]string{"fits": "e1"}) {
		t.Fatalf("later order should still fit after failed append: assignments=%v, unassigned=%+v", actual, got.Unassigned)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Legs) != 1 || !got.Routes[0].Legs[0].StartAt.Equal(at(6, 0)) {
		t.Fatalf("failed candidate must not advance route time/location: %+v", got.Routes)
	}
}

func TestBaselineRequiresAllSkillsAndMatchingTransportOnSameEngineer(t *testing.T) {
	t.Run("all required skills", func(t *testing.T) {
		in := baseRequest()
		in.Orders[0].RequiredSkills = []string{"repair", "install"}
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(assignedIDs(got)) != 0 || len(got.Unassigned) != 1 {
			t.Fatalf("engineer missing one required skill must not receive the order: %+v", got)
		}
	})

	t.Run("skill and transport belong to same engineer", func(t *testing.T) {
		in := baseRequest()
		walk := contracts.TransportWalk
		in.Orders[0].RequiredTransport = &walk
		in.Engineers = []contracts.Engineer{engineer("car-repair", 1), engineer("walk-install", 2)}
		in.Engineers[1].Skills = []string{"install"}
		in.Engineers[1].Transport = contracts.TransportWalk
		in.EngineerStates = []contracts.EngineerState{
			{EngineerID: "car-repair", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
			{EngineerID: "walk-install", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
		}
		in.Orders[0].RequiredSkills = []string{"repair"}
		in.TravelMatrix = matrix([]string{"depot", "p1"}, contracts.TransportCar)
		walkMatrix := matrix([]string{"depot", "p1"}, contracts.TransportWalk)
		in.TravelMatrix.Profiles[contracts.TransportWalk] = walkMatrix.Profiles[contracts.TransportWalk]
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(assignedIDs(got)) != 0 || len(got.Unassigned) != 1 {
			t.Fatalf("skill on one engineer and transport on another must not be combined: %+v", got)
		}
	})
}

func TestBaselineSelectsEngineersTravelProfile(t *testing.T) {
	in := baseRequest()
	in.Orders = []contracts.Order{order("car-order", "car-stop", 1, 7, 0, 10, 0), order("walk-order", "walk-stop", 2, 7, 0, 10, 0)}
	in.Orders[0].RequiredTransport = transportPtr(contracts.TransportCar)
	in.Orders[1].RequiredTransport = transportPtr(contracts.TransportWalk)
	in.Engineers = []contracts.Engineer{engineer("car", 1), engineer("walk", 2)}
	in.Engineers[1].Transport = contracts.TransportWalk
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "car", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
		{EngineerID: "walk", StartLocationID: "depot", AvailableFrom: at(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
	}
	in.TravelMatrix = matrix([]string{"depot", "car-stop", "walk-stop"}, contracts.TransportCar)
	walkMatrix := matrix([]string{"depot", "car-stop", "walk-stop"}, contracts.TransportWalk)
	carCells := in.TravelMatrix.Profiles[contracts.TransportCar]
	walkCells := walkMatrix.Profiles[contracts.TransportWalk]
	carCells[0][1] = travelCell(60, 100)
	walkCells[0][2] = travelCell(1200, 300)
	in.TravelMatrix.Profiles[contracts.TransportCar] = carCells
	in.TravelMatrix.Profiles[contracts.TransportWalk] = walkCells

	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if actual := assignedIDs(got); !reflect.DeepEqual(actual, map[string]string{"car-order": "car", "walk-order": "walk"}) {
		t.Fatalf("unexpected profile-based assignments: %v", actual)
	}
	for _, route := range got.Routes {
		want := int64(100)
		if route.EngineerID == "walk" {
			want = 300
			if !route.Visits[0].ArrivalAt.Equal(at(6, 20)) {
				t.Fatalf("walk profile duration not used: %+v", route.Visits[0])
			}
		} else if !route.Visits[0].ArrivalAt.Equal(at(6, 1)) {
			t.Fatalf("car profile duration not used: %+v", route.Visits[0])
		}
		if route.Legs[0].DistanceM != want {
			t.Fatalf("%s distance = %d, want profile distance %d", route.EngineerID, route.Legs[0].DistanceM, want)
		}
	}
}

func TestBaselineDoesNotReorderEarlierVisits(t *testing.T) {
	in := baseRequest()
	first := order("late-window", "p1", 1, 8, 0, 10, 0)
	second := order("early-window", "p2", 2, 7, 0, 10, 0)
	second.Window.End = at(7, 30)
	in.Orders = []contracts.Order{first, second}
	in.TravelMatrix = matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if actual := assignedIDs(got); !reflect.DeepEqual(actual, map[string]string{"late-window": "e1"}) {
		t.Fatalf("baseline should keep first visit and leave second unassigned: %v", actual)
	}
	if len(got.Unassigned) != 1 || got.Unassigned[0].OrderID != "early-window" || got.Unassigned[0].ReasonCode != contracts.ReasonNotAssignedBySolver {
		t.Fatalf("append-only failure should be generic: %+v", got.Unassigned)
	}
}

func TestBaselineDistinguishesDisconnectedFromMissingDirectEdge(t *testing.T) {
	t.Run("disconnected vertex", func(t *testing.T) {
		in := baseRequest()
		in.TravelMatrix = matrix([]string{"depot", "p1"}, contracts.TransportCar)
		cells := in.TravelMatrix.Profiles[contracts.TransportCar]
		cells[0][1] = unreachableCell()
		in.TravelMatrix.Profiles[contracts.TransportCar] = cells
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Unassigned) != 1 || got.Unassigned[0].ReasonCode != contracts.ReasonNoReachableRoute {
			t.Fatalf("disconnected order reason = %+v, want NO_REACHABLE_ROUTE", got.Unassigned)
		}
	})

	t.Run("indirect path exists", func(t *testing.T) {
		in := baseRequest()
		in.TravelMatrix = matrix([]string{"depot", "p1", "mid"}, contracts.TransportCar)
		cells := in.TravelMatrix.Profiles[contracts.TransportCar]
		cells[0][1] = unreachableCell()
		in.TravelMatrix.Profiles[contracts.TransportCar] = cells
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Unassigned) != 1 || got.Unassigned[0].ReasonCode != contracts.ReasonNotAssignedBySolver {
			t.Fatalf("a possible indirect path means direct append failure is inconclusive: %+v", got.Unassigned)
		}
	})
}

func TestBaselineServiceMayEndAtShiftEndButNotAfter(t *testing.T) {
	for _, tc := range []struct {
		name     string
		service  int64
		assigned bool
	}{
		{"ends exactly at shift end", 20 * 60, true},
		{"ends one second after shift", 20*60 + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := baseRequest()
			in.Orders[0].Window.Start = at(7, 0)
			in.Orders[0].Window.End = at(7, 0)
			in.Orders[0].ServiceSec = tc.service
			in.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = ptr(600)
			in.Engineers[0].Shift.End = at(7, 20)
			got, err := solve(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if assigned := len(assignedIDs(got)) == 1; assigned != tc.assigned {
				t.Fatalf("assigned=%v, want %v; result=%+v", assigned, tc.assigned, got)
			}
		})
	}
}

func TestBaselineEmptyResultUsesEmptyArraysAndNoEngineersReason(t *testing.T) {
	t.Run("empty request serializes arrays", func(t *testing.T) {
		in := baseRequest()
		in.Orders = []contracts.Order{}
		in.Engineers = []contracts.Engineer{}
		in.EngineerStates = []contracts.EngineerState{}
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &raw); err != nil {
			t.Fatal(err)
		}
		if string(raw["routes"]) != "[]" || string(raw["unassigned"]) != "[]" {
			t.Fatalf("empty slices must serialize as arrays: %s", encoded)
		}
	})

	t.Run("orders with no engineers", func(t *testing.T) {
		in := baseRequest()
		in.Engineers = []contracts.Engineer{}
		in.EngineerStates = []contracts.EngineerState{}
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Routes) != 0 || len(got.Unassigned) != 1 || got.Unassigned[0].ReasonCode != contracts.ReasonNoAvailableEngineer {
			t.Fatalf("unexpected no-engineer result: %+v", got)
		}
	})
}

func transportPtr(v contracts.Transport) *contracts.Transport { return &v }

func travelCell(duration, distance int64) contracts.TravelCell {
	return contracts.TravelCell{Reachable: true, DurationSec: &duration, DistanceM: &distance}
}

func unreachableCell() contracts.TravelCell {
	return contracts.TravelCell{Reachable: false}
}
