package baseline_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/baseline"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

func solve(t *testing.T, in contracts.SolveRequest) (contracts.SolveResult, error) {
	t.Helper()
	return baseline.New().Solve(context.Background(), in)
}

func requireInvalid(t *testing.T, in contracts.SolveRequest) {
	t.Helper()
	_, err := solve(t, in)
	var ce *contracts.ContractError
	if !errors.As(err, &ce) || ce.Code != "INVALID_INPUT" {
		t.Fatalf("Solve() error = %v, want ContractError INVALID_INPUT", err)
	}
}

func TestBaselineBackendFlowExample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "docs", "contracts", "examples", "backend_flow.json")
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
	if r.EngineerID != "eng-1" || r.StartLocationID != "office-1" || !r.StartAt.Equal(testutil.At(7, 0)) || len(r.Visits) != 1 || len(r.Legs) != 1 {
		t.Fatalf("unexpected route: %+v", r)
	}
	v := r.Visits[0]
	if v.OrderID != "order-1" || !v.ArrivalAt.Equal(testutil.At(7, 15)) || !v.StartAt.Equal(testutil.At(8, 0)) || !v.EndAt.Equal(testutil.At(8, 30)) {
		t.Fatalf("unexpected visit: %+v", v)
	}
	l := r.Legs[0]
	if l.ID != "leg-1" || l.FromLocationID != "office-1" || l.ToLocationID != "loc-1" || !l.StartAt.Equal(testutil.At(7, 0)) || !l.EndAt.Equal(testutil.At(7, 15)) || l.DistanceM != 1200 || l.GeoContextID != "geo-1" {
		t.Fatalf("unexpected leg: %+v", l)
	}
}

func TestBaselineOrdersAndEngineersUseSourceOrderThenID(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders = []contracts.Order{testutil.Order("z", "p1", 1, 7, 0, 10, 0), testutil.Order("b", "p2", 1, 7, 0, 10, 0), testutil.Order("a", "p3", 1, 7, 0, 10, 0)}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2", "p3"}, contracts.TransportCar)
	in.Engineers = []contracts.Engineer{testutil.Engineer("z-eng", 1), testutil.Engineer("a-eng", 1)}
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "z-eng", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
		{EngineerID: "a-eng", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
	}
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a": "a-eng", "b": "a-eng", "z": "a-eng"}
	if actual := testutil.AssignedIDs(got); !reflect.DeepEqual(actual, want) {
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
	in := testutil.BaseRequest()
	normal := testutil.Order("normal", "p1", 1, 7, 0, 10, 0)
	urgent := testutil.Order("urgent", "p2", 2, 7, 0, 10, 0)
	urgent.WorkType, urgent.Priority, urgent.ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
	in.Orders = []contracts.Order{urgent, normal}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 2 || got.Routes[0].Visits[0].OrderID != "normal" || got.Routes[0].Visits[1].OrderID != "urgent" {
		t.Fatalf("baseline should follow source order despite priority: %+v", got.Routes)
	}
}

func TestBaselineEmergencyFirstPlacesEmergencyBeforeEarlierNormalWork(t *testing.T) {
	in := testutil.BaseRequest()
	in.EmergencyFirst = true
	normal := testutil.Order("normal", "p1", 1, 7, 0, 30, 0)
	emergency := testutil.Order("emergency", "p2", 2, 7, 0, 80, 0)
	emergency.WorkType, emergency.Priority, emergency.ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
	emergency.Window.End = testutil.At(8, 30) // An explicitly widened emergency window.
	in.Orders = []contracts.Order{normal, emergency}
	in.Engineers[0].Shift.End = testutil.At(8, 30)
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 1 || got.Routes[0].Visits[0].OrderID != "emergency" {
		t.Fatalf("emergency must take the only feasible shift slot: %+v", got)
	}
	if len(got.Unassigned) != 1 || got.Unassigned[0].OrderID != "normal" {
		t.Fatalf("normal order should remain unassigned: %+v", got.Unassigned)
	}
}

func TestBaselineWidenedEmergencyStillRespectsEligibilityAndShift(t *testing.T) {
	cases := map[string]func(*contracts.SolveRequest){
		"skill": func(in *contracts.SolveRequest) { in.Orders[0].RequiredSkills = []string{"special"} },
		"equipment": func(in *contracts.SolveRequest) {
			in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 3}
		},
		"transport": func(in *contracts.SolveRequest) {
			walk := contracts.TransportWalk
			in.Orders[0].RequiredTransport = &walk
		},
		"shift": func(in *contracts.SolveRequest) { in.Engineers[0].Shift.End = testutil.At(8, 0) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := testutil.BaseRequest()
			in.EmergencyFirst = true
			in.Orders[0].WorkType, in.Orders[0].Priority, in.Orders[0].ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
			in.Orders[0].Window.End = testutil.At(12, 0)
			mutate(&in)
			got, err := solve(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Routes) != 0 || len(got.Unassigned) != 1 {
				t.Fatalf("ineligible emergency was assigned: %+v", got)
			}
		})
	}
}

func TestBaselineAllowsWindowEndStartAndFinishAfterWindow(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders[0].Window = contracts.Window{Start: testutil.At(7, 10), End: testutil.At(7, 10)}
	in.Orders[0].ServiceSec = 30 * 60
	in.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = testutil.Ptr(600)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	v := got.Routes[0].Visits[0]
	if !v.StartAt.Equal(testutil.At(7, 10)) || !v.EndAt.Equal(testutil.At(7, 40)) {
		t.Fatalf("visit = %+v; start at window end and finish after it should be allowed", v)
	}
}

func TestBaselineRespectsReceivedAtForDeparture(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders[0].ReceivedAt = testutil.At(8, 0)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !got.Routes[0].Legs[0].StartAt.Equal(testutil.At(8, 0)) || !got.Routes[0].Visits[0].ArrivalAt.Equal(testutil.At(8, 10)) {
		t.Fatalf("received_at must constrain departure to the order: route=%+v", got.Routes[0])
	}
}

func TestBaselineUsesEngineerStateForReplanning(t *testing.T) {
	in := testutil.BaseRequest()
	in.EngineerStates[0].StartLocationID = "p1"
	in.EngineerStates[0].AvailableFrom = testutil.At(8, 0)
	in.Orders[0].LocationID = "p2"
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].StartLocationID != "p1" || !got.Routes[0].StartAt.Equal(testutil.At(8, 0)) || got.Routes[0].Legs[0].FromLocationID != "p1" {
		t.Fatalf("route did not start from live engineer state: %+v", got.Routes)
	}
}

func TestBaselineUsesDirectedProfileTravel(t *testing.T) {
	in := testutil.BaseRequest()
	in.EngineerStates[0].StartLocationID = "p1"
	in.Orders[0].LocationID = "depot"
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1"}, contracts.TransportCar)
	in.TravelMatrix.Profiles[contracts.TransportCar][1][0] = contracts.TravelCell{Reachable: true, DurationSec: testutil.Ptr(120), DistanceM: testutil.Ptr(250)}
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Legs) != 1 || got.Routes[0].Legs[0].FromLocationID != "p1" || got.Routes[0].Legs[0].DistanceM != 250 || !got.Routes[0].Visits[0].ArrivalAt.Equal(testutil.At(6, 2)) {
		t.Fatalf("expected p1→depot directed matrix leg: %+v", got.Routes)
	}
}

func TestBaselineReservesEquipmentFromEngineerStateWithoutMutatingInput(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders = []contracts.Order{testutil.Order("o1", "p1", 1, 7, 0, 10, 0), testutil.Order("o2", "p2", 2, 7, 0, 10, 0)}
	in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders[1].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Engineers[0].EquipmentStock = map[contracts.Equipment]int64{contracts.EquipmentRouter: 99}
	in.EngineerStates[0].EquipmentAvailable = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	before, _ := json.Marshal(in)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(in)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Solve mutated input")
	}
	if len(testutil.AssignedIDs(got)) != 1 || len(got.Unassigned) != 1 {
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
			in := testutil.BaseRequest()
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
			in := testutil.BaseRequest()
			in.Engineers = append(in.Engineers, testutil.Engineer("e2", 2))
			in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "e2", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}})
			in.Orders[0].Status = status
			execution := &contracts.OrderExecution{EngineerID: "e2"}
			if status == contracts.OrderStatusEnRoute {
				departed := testutil.At(6, 0)
				execution.DepartedAt = &departed
			}
			in.Orders[0].Execution = execution
			got, err := solve(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if actual := testutil.AssignedIDs(got)[in.Orders[0].ID]; actual != "e1" {
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
			r.TravelMatrix.Profiles[contracts.TransportCar][0][1] = contracts.TravelCell{Reachable: false, DurationSec: testutil.Ptr(0), DistanceM: testutil.Ptr(0)}
		},
		"negative travel": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = testutil.Ptr(-1)
		},
		"bad diagonal": func(r *contracts.SolveRequest) {
			r.TravelMatrix.Profiles[contracts.TransportCar][0][0].DistanceM = testutil.Ptr(1)
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
		t.Run(name, func(t *testing.T) { in := testutil.BaseRequest(); mutate(&in); requireInvalid(t, in) })
	}
}

func TestBaselineReturnsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := baseline.New().Solve(ctx, testutil.BaseRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Solve() error = %v, want context.Canceled", err)
	}
}

func TestBaselineFailedAppendDoesNotConsumeTimeOrEquipment(t *testing.T) {
	in := testutil.BaseRequest()
	blocked := testutil.Order("too-late", "p1", 1, 7, 0, 20, 0)
	blocked.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	blocked.Window = contracts.Window{Start: testutil.At(17, 55), End: testutil.At(17, 55)}
	laterOrder := testutil.Order("fits", "p2", 2, 7, 0, 20, 0)
	laterOrder.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders = []contracts.Order{blocked, laterOrder}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	in.Engineers[0].Shift.End = testutil.At(18, 0)
	in.Engineers[0].EquipmentStock[contracts.EquipmentRouter] = 1
	in.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 1

	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if actual := testutil.AssignedIDs(got); !reflect.DeepEqual(actual, map[string]string{"fits": "e1"}) {
		t.Fatalf("later order should still fit after failed append: assignments=%v, unassigned=%+v", actual, got.Unassigned)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Legs) != 1 || !got.Routes[0].Legs[0].StartAt.Equal(testutil.At(6, 0)) {
		t.Fatalf("failed candidate must not advance route time/location: %+v", got.Routes)
	}
}

func TestBaselineRequiresAllSkillsAndMatchingTransportOnSameEngineer(t *testing.T) {
	t.Run("all required skills", func(t *testing.T) {
		in := testutil.BaseRequest()
		in.Orders[0].RequiredSkills = []string{"repair", "install"}
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(testutil.AssignedIDs(got)) != 0 || len(got.Unassigned) != 1 {
			t.Fatalf("engineer missing one required skill must not receive the order: %+v", got)
		}
	})

	t.Run("skill and transport belong to same engineer", func(t *testing.T) {
		in := testutil.BaseRequest()
		walk := contracts.TransportWalk
		in.Orders[0].RequiredTransport = &walk
		in.Engineers = []contracts.Engineer{testutil.Engineer("car-repair", 1), testutil.Engineer("walk-install", 2)}
		in.Engineers[1].Skills = []string{"install"}
		in.Engineers[1].Transport = contracts.TransportWalk
		in.EngineerStates = []contracts.EngineerState{
			{EngineerID: "car-repair", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
			{EngineerID: "walk-install", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
		}
		in.Orders[0].RequiredSkills = []string{"repair"}
		in.TravelMatrix = testutil.Matrix([]string{"depot", "p1"}, contracts.TransportCar)
		walkMatrix := testutil.Matrix([]string{"depot", "p1"}, contracts.TransportWalk)
		in.TravelMatrix.Profiles[contracts.TransportWalk] = walkMatrix.Profiles[contracts.TransportWalk]
		got, err := solve(t, in)
		if err != nil {
			t.Fatal(err)
		}
		if len(testutil.AssignedIDs(got)) != 0 || len(got.Unassigned) != 1 {
			t.Fatalf("skill on one engineer and transport on another must not be combined: %+v", got)
		}
	})
}

func TestBaselineSelectsEngineersTravelProfile(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders = []contracts.Order{testutil.Order("car-order", "car-stop", 1, 7, 0, 10, 0), testutil.Order("walk-order", "walk-stop", 2, 7, 0, 10, 0)}
	in.Orders[0].RequiredTransport = testutil.TransportPtr(contracts.TransportCar)
	in.Orders[1].RequiredTransport = testutil.TransportPtr(contracts.TransportWalk)
	in.Engineers = []contracts.Engineer{testutil.Engineer("car", 1), testutil.Engineer("walk", 2)}
	in.Engineers[1].Transport = contracts.TransportWalk
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "car", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
		{EngineerID: "walk", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{}},
	}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "car-stop", "walk-stop"}, contracts.TransportCar)
	walkMatrix := testutil.Matrix([]string{"depot", "car-stop", "walk-stop"}, contracts.TransportWalk)
	carCells := in.TravelMatrix.Profiles[contracts.TransportCar]
	walkCells := walkMatrix.Profiles[contracts.TransportWalk]
	carCells[0][1] = testutil.TravelCell(60, 100)
	walkCells[0][2] = testutil.TravelCell(1200, 300)
	in.TravelMatrix.Profiles[contracts.TransportCar] = carCells
	in.TravelMatrix.Profiles[contracts.TransportWalk] = walkCells

	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if actual := testutil.AssignedIDs(got); !reflect.DeepEqual(actual, map[string]string{"car-order": "car", "walk-order": "walk"}) {
		t.Fatalf("unexpected profile-based assignments: %v", actual)
	}
	for _, route := range got.Routes {
		want := int64(100)
		if route.EngineerID == "walk" {
			want = 300
			if !route.Visits[0].ArrivalAt.Equal(testutil.At(6, 20)) {
				t.Fatalf("walk profile duration not used: %+v", route.Visits[0])
			}
		} else if !route.Visits[0].ArrivalAt.Equal(testutil.At(6, 1)) {
			t.Fatalf("car profile duration not used: %+v", route.Visits[0])
		}
		if route.Legs[0].DistanceM != want {
			t.Fatalf("%s distance = %d, want profile distance %d", route.EngineerID, route.Legs[0].DistanceM, want)
		}
	}
}

func TestBaselineDoesNotReorderEarlierVisits(t *testing.T) {
	in := testutil.BaseRequest()
	first := testutil.Order("late-window", "p1", 1, 8, 0, 10, 0)
	second := testutil.Order("early-window", "p2", 2, 7, 0, 10, 0)
	second.Window.End = testutil.At(7, 30)
	in.Orders = []contracts.Order{first, second}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	got, err := solve(t, in)
	if err != nil {
		t.Fatal(err)
	}
	if actual := testutil.AssignedIDs(got); !reflect.DeepEqual(actual, map[string]string{"late-window": "e1"}) {
		t.Fatalf("baseline should keep first visit and leave second unassigned: %v", actual)
	}
	if len(got.Unassigned) != 1 || got.Unassigned[0].OrderID != "early-window" || got.Unassigned[0].ReasonCode != contracts.ReasonNotAssignedBySolver {
		t.Fatalf("append-only failure should be generic: %+v", got.Unassigned)
	}
}

func TestBaselineDistinguishesDisconnectedFromMissingDirectEdge(t *testing.T) {
	t.Run("disconnected vertex", func(t *testing.T) {
		in := testutil.BaseRequest()
		in.TravelMatrix = testutil.Matrix([]string{"depot", "p1"}, contracts.TransportCar)
		cells := in.TravelMatrix.Profiles[contracts.TransportCar]
		cells[0][1] = testutil.UnreachableCell()
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
		in := testutil.BaseRequest()
		in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "mid"}, contracts.TransportCar)
		cells := in.TravelMatrix.Profiles[contracts.TransportCar]
		cells[0][1] = testutil.UnreachableCell()
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
			in := testutil.BaseRequest()
			in.Orders[0].Window.Start = testutil.At(7, 0)
			in.Orders[0].Window.End = testutil.At(7, 0)
			in.Orders[0].ServiceSec = tc.service
			in.TravelMatrix.Profiles[contracts.TransportCar][0][1].DurationSec = testutil.Ptr(600)
			in.Engineers[0].Shift.End = testutil.At(7, 20)
			got, err := solve(t, in)
			if err != nil {
				t.Fatal(err)
			}
			if assigned := len(testutil.AssignedIDs(got)) == 1; assigned != tc.assigned {
				t.Fatalf("assigned=%v, want %v; result=%+v", assigned, tc.assigned, got)
			}
		})
	}
}

func TestBaselineEmptyResultUsesEmptyArraysAndNoEngineersReason(t *testing.T) {
	t.Run("empty request serializes arrays", func(t *testing.T) {
		in := testutil.BaseRequest()
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
		in := testutil.BaseRequest()
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
