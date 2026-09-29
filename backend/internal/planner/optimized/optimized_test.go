//go:build vroom

package optimized_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

// These integration tests require the VROOM executable. The default suite
// tests the adapter and process handling without a native solver installation.
func TestOptimizedPrioritizesEmergencyAndDoesNotMutateInput(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders = []contracts.Order{
		testutil.Order("repair", "p1", 1, 6, 0, 1, 0),
		testutil.Order("emergency", "p1", 2, 6, 0, 80, 0),
	}
	in.Orders[1].WorkType = contracts.WorkTypeEmergency
	in.Orders[1].Priority = contracts.PriorityUrgent
	in.Orders[1].ServiceSec = 4800
	in.Engineers[0].Shift.End = testutil.At(7, 30)
	beforeBytes, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Unassigned) != 1 || got.Unassigned[0].OrderID != "repair" || !hasVisit(got, "emergency") {
		t.Fatalf("emergency should dominate repair when only one slot fits: %+v", got)
	}
	afterBytes, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeBytes) != string(afterBytes) {
		t.Fatal("optimized solver mutated input")
	}
}

func TestOptimizedWidenedEmergencyKeepsHardConstraints(t *testing.T) {
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
			in.Mode = contracts.SolveModeOptimized
			in.EmergencyFirst = true
			in.Orders[0].WorkType, in.Orders[0].Priority, in.Orders[0].ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
			in.Orders[0].Window.End = testutil.At(12, 0)
			mutate(&in)
			got, err := planner.New().Solve(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Routes) != 0 || len(got.Unassigned) != 1 {
				t.Fatalf("ineligible emergency was assigned: %+v", got)
			}
		})
	}
}

func TestOptimizedEmergencyFirstPrefersEarlierArrivalOverOrdinaryAssignment(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.TimeLimitMS = 4000
	in.Orders = []contracts.Order{
		testutil.Order("normal", "p1", 1, 6, 0, 60, 0),
		testutil.Order("emergency", "p2", 2, 7, 0, 80, 7),
	}
	in.Orders[0].Window.End = testutil.At(8, 30)
	in.Orders[1].WorkType, in.Orders[1].Priority, in.Orders[1].ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
	in.Orders[1].Window.End = testutil.At(10, 0) // Late eligibility was established by the caller.
	in.Engineers[0].Shift.End = testutil.At(10, 0)
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)

	ordinaryFirst, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(testutil.AssignedIDs(ordinaryFirst)) != 2 {
		t.Fatalf("control schedule should fit both orders: %+v", ordinaryFirst)
	}
	in.EmergencyFirst = true
	emergencyFirst, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(emergencyFirst.Routes) != 1 || len(emergencyFirst.Routes[0].Visits) != 1 || emergencyFirst.Routes[0].Visits[0].OrderID != "emergency" || !emergencyFirst.Routes[0].Visits[0].StartAt.Equal(testutil.At(7, 10)) {
		t.Fatalf("emergency should start at the earliest feasible time before normal work is considered: %+v", emergencyFirst)
	}
	if len(emergencyFirst.Unassigned) != 1 || emergencyFirst.Unassigned[0].OrderID != "normal" {
		t.Fatalf("displaced normal order should be explained: %+v", emergencyFirst.Unassigned)
	}
}

func TestOptimizedEnforcesReceivedAtAndEligibility(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders[0].ReceivedAt = testutil.At(8, 0)
	in.Orders[0].Window.Start = testutil.At(6, 0)
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !hasVisit(got, "o1") {
		t.Fatalf("feasible order was not assigned: %+v", got)
	}
	for _, route := range got.Routes {
		for _, visit := range route.Visits {
			if visit.OrderID == "o1" {
				if visit.StartAt.Before(in.Orders[0].ReceivedAt) {
					t.Fatalf("service starts before received_at: %+v", visit)
				}
				foundLeg := false
				for _, leg := range route.Legs {
					if leg.ToLocationID == "p1" {
						foundLeg = true
						if leg.StartAt.Before(in.Orders[0].ReceivedAt) {
							t.Fatalf("departure before received_at: %+v", leg)
						}
					}
				}
				if !foundLeg {
					t.Fatalf("missing leg to order: %+v", route)
				}
			}
		}
	}
}

func TestOptimizedRejectsUnreachableRoute(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	cell := in.TravelMatrix.Profiles[contracts.TransportCar][0][1]
	cell.Reachable, cell.DurationSec, cell.DistanceM = false, nil, nil
	in.TravelMatrix.Profiles[contracts.TransportCar][0][1] = cell
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Unassigned) != 1 || got.Unassigned[0].ReasonCode != contracts.ReasonNoReachableRoute {
		t.Fatalf("unreachable order should have a proven route reason: %+v", got.Unassigned)
	}
}

func TestOptimizedCancellationReturnsNoPlan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	if _, err := planner.New().Solve(ctx, in); err == nil {
		t.Fatal("cancelled context should be returned to caller")
	}
}

func TestOptimizedConnectionCountDominatesRepairs(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders = []contracts.Order{
		testutil.Order("r1", "p1", 1, 6, 0, 5, 0),
		testutil.Order("r2", "p2", 2, 6, 0, 5, 0),
		testutil.Order("c1", "p3", 3, 6, 0, 20, 0),
	}
	in.Orders[2].WorkType = contracts.WorkTypeConnection
	in.Engineers[0].Shift.End = testutil.At(6, 40)
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2", "p3"}, contracts.TransportCar)
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !hasVisit(got, "c1") {
		t.Fatalf("connection must be retained ahead of repairs: %+v", got)
	}
}

func TestOptimizedEmergencyDelayPrecedesNewEngineerCount(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders[0].WorkType = contracts.WorkTypeEmergency
	in.Orders[0].Priority = contracts.PriorityUrgent
	in.Orders[0].ServiceSec = 4800
	in.AlreadyUsedEngineerIDs = []string{"e2"}
	in.Orders[0].ReceivedAt = testutil.At(7, 0)
	in.Orders[0].Window = contracts.Window{Start: testutil.At(7, 0), End: testutil.At(12, 0)}
	in.Engineers = []contracts.Engineer{testutil.Engineer("e1", 2), testutil.Engineer("e2", 1)}
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "e1", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}},
		{EngineerID: "e2", StartLocationID: "p1", AvailableFrom: testutil.At(8, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}},
	}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1"}, contracts.TransportCar)
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].EngineerID != "e1" {
		t.Fatalf("earliest emergency assignment should win before engineer count: %+v", got.Routes)
	}
}

func TestOptimizedAlreadyUsedEngineerPrecedesDistance(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.AlreadyUsedEngineerIDs = []string{"e1"}
	in.Engineers = []contracts.Engineer{testutil.Engineer("e1", 2), testutil.Engineer("e2", 1)}
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "e1", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}},
		{EngineerID: "e2", StartLocationID: "p1", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}},
	}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1"}, contracts.TransportCar)
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].EngineerID != "e1" {
		t.Fatalf("already-used engineer should win before distance: %+v", got.Routes)
	}
}

func hasVisit(result contracts.SolveResult, id string) bool {
	for _, route := range result.Routes {
		for _, visit := range route.Visits {
			if visit.OrderID == id {
				return true
			}
		}
	}
	return false
}

func TestOptimizedDistanceReordersBaseline(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders = []contracts.Order{testutil.Order("far", "p1", 1, 6, 0, 10, 0), testutil.Order("near", "p2", 2, 6, 0, 10, 0)}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	cells := in.TravelMatrix.Profiles[contracts.TransportCar]
	cells[0][1].DistanceM = testutil.Ptr(10000)
	cells[1][2].DistanceM = testutil.Ptr(10000)
	cells[0][2].DistanceM = testutil.Ptr(10)
	cells[2][1].DistanceM = testutil.Ptr(10)
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 2 || got.Routes[0].Visits[0].OrderID != "near" {
		t.Fatalf("distance stage did not improve route: %+v", got)
	}
}

func TestOptimizedUsesSkillsTransportEquipmentAndOpenRoutes(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders = []contracts.Order{testutil.Order("a", "p1", 1, 6, 0, 10, 0), testutil.Order("b", "p1", 2, 6, 0, 10, 0), testutil.Order("wrong-skill", "p1", 3, 6, 0, 10, 0), testutil.Order("walk-job", "p2", 4, 6, 0, 10, 0)}
	in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders[1].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders[2].RequiredSkills = []string{"repair", "missing"}
	walk := contracts.TransportWalk
	in.Orders[3].RequiredTransport = &walk
	in.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 1
	walker := testutil.Engineer("walker", 2)
	walker.Transport = walk
	in.Engineers = append(in.Engineers, walker)
	in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "walker", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0)})
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	in.TravelMatrix.Profiles[walk] = testutil.Matrix(in.TravelMatrix.LocationIDs, walk).Profiles[walk]
	// There is no return path; open routes must still be serviceable.
	for _, profile := range in.TravelMatrix.Profiles {
		profile[1][0] = contracts.TravelCell{}
		profile[2][0] = contracts.TravelCell{}
	}
	in.TravelMatrix.Profiles[contracts.TransportCar][0][2] = contracts.TravelCell{}
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	ids := testutil.AssignedIDs(got)
	if len(ids) != 2 || ids["walk-job"] != "walker" || ids["wrong-skill"] != "" {
		t.Fatalf("eligibility/open route violated: %+v", got)
	}
	if (ids["a"] != "") == (ids["b"] != "") {
		t.Fatalf("exactly one equipment order must fit: %v", ids)
	}
}

func TestOptimizedReceivedAtConstrainsTravelNotJustService(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.TimeLimitMS = 300
	in.Orders[0].ReceivedAt = testutil.At(8, 0)
	in.Orders[0].Window = contracts.Window{Start: testutil.At(8, 5), End: testutil.At(8, 5)}
	// Ten-minute travel cannot start before receipt, even though an 08:05 service
	// start would satisfy a plain customer time window.
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 0 || len(got.Unassigned) != 1 {
		t.Fatalf("illegal pre-receipt departure accepted: %+v", got)
	}
}

func TestOptimizedContextCancelsNativeSearch(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.TimeLimitMS = 10000
	in.Orders = []contracts.Order{testutil.Order("far", "p1", 1, 6, 0, 10, 0), testutil.Order("near", "p2", 2, 6, 0, 10, 0)}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	got, err := planner.New().Solve(ctx, in)
	if !errors.Is(err, context.DeadlineExceeded) || len(got.Routes) != 0 {
		t.Fatalf("cancellation: result=%+v err=%v", got, err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("native search did not stop promptly")
	}
}

func TestOptimizedTinyBudgetKeepsEveryOrder(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.TimeLimitMS = 1
	in.Orders = []contracts.Order{testutil.Order("a", "p1", 1, 6, 0, 10, 0), testutil.Order("b", "p1", 2, 6, 0, 10, 0)}
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, r := range got.Routes {
		for _, v := range r.Visits {
			seen[v.OrderID]++
		}
	}
	for _, u := range got.Unassigned {
		seen[u.OrderID]++
	}
	if len(seen) != 2 || seen["a"] != 1 || seen["b"] != 1 {
		t.Fatalf("lost/duplicate orders: %+v", got)
	}
}

func TestOptimizedEngineerCountPrecedesDistance(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders = []contracts.Order{testutil.Order("a", "p1", 1, 6, 0, 10, 0), testutil.Order("b", "p2", 2, 6, 0, 10, 0)}
	in.Orders[1].RequiredSkills = []string{"extra"}
	in.Engineers = []contracts.Engineer{testutil.Engineer("e1", 1), testutil.Engineer("e2", 2)}
	in.Engineers[1].Skills = []string{"repair", "extra"}
	in.EngineerStates = []contracts.EngineerState{
		{EngineerID: "e1", StartLocationID: "p1", AvailableFrom: testutil.At(6, 0)},
		{EngineerID: "e2", StartLocationID: "p2", AvailableFrom: testutil.At(6, 0)},
	}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	// Two engineers can each serve their local job with zero travel. One engineer
	// serving both jobs takes a positive-distance trip, but has higher priority.
	got, err := planner.New().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || got.Routes[0].EngineerID != "e2" || len(got.Routes[0].Visits) != 2 || len(got.Unassigned) != 0 {
		t.Fatalf("distance overrode engineer count: %+v", got)
	}
}
