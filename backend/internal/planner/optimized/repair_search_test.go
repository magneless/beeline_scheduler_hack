package optimized

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

func TestRepairRestoresFeasibilityAfterRemovingShortcut(t *testing.T) {
	in := testutil.BaseRequest()
	in.Engineers[0].Shift.End = testutil.At(7, 0)
	in.Orders = []contracts.Order{testutil.Order("a", "a", 1, 6, 0, 10, 0), testutil.Order("b", "b", 2, 6, 0, 10, 0), testutil.Order("c", "c", 3, 6, 0, 10, 0)}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "a", "b", "c"}, contracts.TransportCar)
	in.TravelMatrix.Profiles[contracts.TransportCar][1][3] = testutil.TravelCell(7200, 10000)
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.materialize([][]int{{0, 1, 2}}); err != nil {
		t.Fatal(err)
	}
	// A -> C is too long after removing B, despite the original route being feasible.
	seq := p.feasiblePrefixes([][]int{{0, 2}})
	if !reflect.DeepEqual(seq, [][]int{{0}}) {
		t.Fatalf("invalid tail retained: %v", seq)
	}
	seq = p.repair(context.Background(), seq, rand.New(rand.NewSource(1)), time.Now().Add(time.Second), 0)
	got, err := p.materialize(seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Unassigned) != 0 || len(got.Routes[0].Visits) != 3 {
		t.Fatalf("repair failed to restore the feasible chain: %+v", got)
	}
}

func TestRepairSearchKeepsLexicographicPrioritiesAndInput(t *testing.T) {
	in := testutil.BaseRequest()
	in.Engineers[0].Shift.End = testutil.At(7, 30)
	in.Orders = []contracts.Order{testutil.Order("repair", "p1", 1, 6, 0, 60, 0), testutil.Order("connection", "p1", 2, 6, 0, 70, 0), testutil.Order("emergency", "p1", 3, 6, 0, 80, 0)}
	in.Orders[1].WorkType = contracts.WorkTypeConnection
	in.Orders[2].WorkType = contracts.WorkTypeEmergency
	in.Orders[2].Priority = contracts.PriorityUrgent
	before, _ := json.Marshal(in)
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := p.materialize([][]int{{0}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.repairSearch(context.Background(), seed, time.Now().Add(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if p.score(got).counts[0] != 1 || len(got.Unassigned) != 2 {
		t.Fatalf("lower-priority work displaced emergency: %+v", got)
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("input was mutated")
	}
	// An expired budget must retain the incumbent, including its assignments.
	unchanged, err := p.repairSearch(context.Background(), got, time.Now().Add(-time.Second))
	if err != nil || !reflect.DeepEqual(got, unchanged) {
		t.Fatal("timeout lost incumbent")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.repairSearch(ctx, got, time.Now().Add(time.Second))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}

func TestRepairNeverSpendsEquipmentTwiceOrViolatesEligibility(t *testing.T) {
	in := testutil.BaseRequest()
	in.EngineerStates[0].EquipmentAvailable = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders = []contracts.Order{
		testutil.Order("stock-a", "p1", 1, 6, 0, 10, 0), testutil.Order("stock-b", "p1", 2, 6, 0, 10, 0),
		testutil.Order("skill", "p1", 3, 6, 0, 10, 0), testutil.Order("transport", "p1", 4, 6, 0, 10, 0),
		testutil.Order("late-receipt", "p1", 5, 6, 0, 10, 18), testutil.Order("closed-window", "p1", 6, 5, 0, 10, 0),
	}
	for i := 0; i < 2; i++ {
		in.Orders[i].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	}
	in.Orders[2].RequiredSkills = []string{"missing"}
	in.Orders[3].RequiredTransport = testutil.TransportPtr(contracts.TransportWalk)
	in.Orders[5].Window.End = testutil.At(5, 59)
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.repairSearch(context.Background(), p.emptyResult(), time.Now().Add(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 1 || len(got.Unassigned) != 5 {
		t.Fatalf("hard constraint violated: %+v", got)
	}
	if id := got.Routes[0].Visits[0].OrderID; id != "stock-a" && id != "stock-b" {
		t.Fatalf("ineligible order assigned: %s", id)
	}
	seen := map[string]bool{got.Routes[0].Visits[0].OrderID: true}
	for _, o := range got.Unassigned {
		if seen[o.OrderID] {
			t.Fatal("duplicate order")
		}
		seen[o.OrderID] = true
	}
	if len(seen) != len(in.Orders) {
		t.Fatal("order lost")
	}
}

func TestRepairEmergencyFirstDoesNotTradeArrivalForMoreOrdinaryWork(t *testing.T) {
	in := testutil.BaseRequest()
	in.EmergencyFirst = true
	in.Orders = []contracts.Order{
		testutil.Order("emergency", "p1", 1, 6, 0, 80, 0),
		testutil.Order("repair", "p2", 2, 6, 0, 10, 0),
	}
	in.Orders[0].WorkType = contracts.WorkTypeEmergency
	in.Orders[0].Priority = contracts.PriorityUrgent
	in.Orders[1].Window.End = testutil.At(6, 30)
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2"}, contracts.TransportCar)
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := p.materialize([][]int{{0}})
	if err != nil {
		t.Fatal(err)
	}
	// Both jobs fit, but the repair would delay arrival at the emergency.
	if _, err := p.materialize([][]int{{1, 0}}); err != nil {
		t.Fatal(err)
	}
	got, err := p.repairSearch(context.Background(), seed, time.Now().Add(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, seed) {
		t.Fatalf("repair changed the earliest emergency arrival: %+v", got)
	}
}
