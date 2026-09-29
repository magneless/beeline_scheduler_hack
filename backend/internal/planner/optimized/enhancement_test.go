package optimized

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

func TestFastEstimateMatchesReferenceAcrossConstraints(t *testing.T) {
	in := testutil.BaseRequest()
	in.Engineers = append(in.Engineers, testutil.Engineer("walker", 2))
	in.Engineers[1].Transport = contracts.TransportWalk
	in.Engineers[1].Skills = []string{"repair", "emergency"}
	in.Engineers[1].EquipmentStock[contracts.EquipmentTVBox] = 1
	in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "walker", StartLocationID: "depot", AvailableFrom: testutil.At(7, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentTVBox: 1}})
	in.Orders = []contracts.Order{
		testutil.Order("a", "a", 1, 7, 0, 30, 0), testutil.Order("b", "b", 2, 7, 0, 60, 9),
		testutil.Order("c", "c", 3, 8, 0, 80, 7), testutil.Order("d", "d", 4, 6, 0, 20, 0),
		testutil.Order("e", "e", 5, 17, 0, 70, 0), testutil.Order("f", "a", 6, 6, 0, 10, 0),
	}
	in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}
	in.Orders[1].RequiredTransport = testutil.TransportPtr(contracts.TransportWalk)
	in.Orders[1].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentTVBox: 1}
	in.Orders[2].WorkType = contracts.WorkTypeEmergency
	in.Orders[2].RequiredSkills = []string{"emergency"}
	in.Orders[2].ReceivedAt = in.Orders[2].ReceivedAt.Add(750 * time.Millisecond)
	in.Orders[2].Window.Start = in.Orders[2].Window.Start.Add(250 * time.Millisecond)
	in.Orders[3].Window.End = testutil.At(6, 15)
	in.Orders[5].RequiredSkills = []string{"missing"}
	in.TravelMatrix = testutil.Matrix([]string{"depot", "a", "b", "c", "d", "e"}, contracts.TransportCar)
	in.TravelMatrix.Profiles[contracts.TransportWalk] = testutil.Matrix(in.TravelMatrix.LocationIDs, contracts.TransportWalk).Profiles[contracts.TransportWalk]
	in.TravelMatrix.Profiles[contracts.TransportCar][1][2] = testutil.UnreachableCell()
	in.TravelMatrix.Profiles[contracts.TransportWalk][3][2] = testutil.TravelCell(1801, 2300)
	in.TravelMatrix.Profiles[contracts.TransportWalk][2][3] = testutil.TravelCell(299, 450)
	before, _ := json.Marshal(in)
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	p.prepareFastEstimate()
	rng := rand.New(rand.NewSource(81))
	for sample := 0; sample < 2000; sample++ {
		seq := rng.Perm(len(in.Orders))[:rng.Intn(len(in.Orders)+1)]
		for v := range p.workers {
			want, got := p.referenceEstimate(v, seq), p.fastEstimate(v, seq)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("worker=%d seq=%v reference=%+v fast=%+v", v, seq, want, got)
			}
		}
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("estimation changed input stock or orders")
	}
}

func enhancementProblem(t *testing.T, demand int64) *routingProblem {
	t.Helper()
	in := testutil.BaseRequest()
	in.Orders = []contracts.Order{testutil.Order("a", "a", 1, 7, 0, 10, 0), testutil.Order("b", "b", 2, 7, 0, 10, 0)}
	for i := range in.Orders {
		in.Orders[i].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: demand}
	}
	in.Engineers = append(in.Engineers, testutil.Engineer("e2", 2))
	in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "e2", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}})
	in.TravelMatrix = testutil.Matrix([]string{"depot", "a", "b"}, contracts.TransportCar)
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	p.prepareFastEstimate()
	return p
}

func TestCrewEliminationDoesNotReopenExcludedCrew(t *testing.T) {
	p := enhancementProblem(t, 1)
	initial, err := p.materialize([][]int{{0}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(initial)
	result := p.eliminateCrew(context.Background(), initial, time.Now().Add(250*time.Millisecond), rand.New(rand.NewSource(1)), 0)
	if len(result.Unassigned) != 0 || len(result.Routes) != 1 || result.Routes[0].EngineerID != "e2" {
		t.Fatalf("excluded crew reused or assignment lost: %+v", result)
	}
	after, _ := json.Marshal(initial)
	if string(before) != string(after) {
		t.Fatal("initial plan mutated")
	}
}

func TestCrewEliminationRetainsPlanWhenStockPreventsMerge(t *testing.T) {
	p := enhancementProblem(t, 2)
	initial, err := p.materialize([][]int{{0}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	result := p.eliminateCrew(context.Background(), initial, time.Now().Add(100*time.Millisecond), rand.New(rand.NewSource(1)), 0)
	if !reflect.DeepEqual(initial, result) {
		t.Fatal("infeasible crew reduction changed best plan")
	}
	p.alreadyUsed["e1"], p.alreadyUsed["e2"] = true, true
	result = p.eliminateCrew(context.Background(), initial, time.Now().Add(100*time.Millisecond), rand.New(rand.NewSource(1)), 0)
	if !reflect.DeepEqual(initial, result) {
		t.Fatal("already-used crew was counted as saved")
	}
}

func TestSegmentExchangeImprovesDistanceWithoutChangingCoverageOrStock(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders = nil
	for i, id := range []string{"a1", "b1", "a2", "b2"} {
		o := testutil.Order(id, id, int64(i+1), 7, 0, 10, 0)
		o.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
		in.Orders = append(in.Orders, o)
	}
	in.Engineers = append(in.Engineers, testutil.Engineer("e2", 2))
	in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "e2", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentRouter: 2}})
	in.TravelMatrix = testutil.Matrix([]string{"depot", "a1", "b1", "a2", "b2"}, contracts.TransportCar)
	for i, row := range in.TravelMatrix.Profiles[contracts.TransportCar] {
		for j := range row {
			if i == j {
				continue
			}
			distance := int64(100)
			if i == 0 || j == 0 {
				distance = 10
			} else if i%2 == j%2 {
				distance = 1
			}
			row[j] = testutil.TravelCell(60, distance)
		}
	}
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	p.prepareFastEstimate()
	initial, err := p.materialize([][]int{{0, 1}, {2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	result := p.improveSegments(context.Background(), initial, time.Now().Add(250*time.Millisecond), true)
	if len(result.Unassigned) != 0 || p.score(result).used != 2 || p.score(result).distance != 22 {
		t.Fatalf("valid 2-crew exchange not found: %+v", p.score(result))
	}
}

func TestEnhancedSearchDeadlineAndCancellationKeepIncumbent(t *testing.T) {
	p := enhancementProblem(t, 1)
	p.search = SearchOptions{FastEstimate: true, CrewElimination: true, Segments: true, Adaptive: true}
	initial, err := p.materialize([][]int{{0}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.enhancedSearch(context.Background(), initial, time.Now().Add(-time.Second), rand.New(rand.NewSource(1)))
	if err != nil || !reflect.DeepEqual(initial, result) {
		t.Fatal("expired deadline lost incumbent")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = p.enhancedSearch(ctx, initial, time.Now().Add(time.Second), rand.New(rand.NewSource(1)))
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(initial, result) {
		t.Fatal("cancellation lost incumbent or was ignored")
	}
}

func TestFastConcurrentDiagnosticsRemainIndependent(t *testing.T) {
	solver := NewWithSearchOptions(SearchOptions{FastEstimate: true, RegretOnly: true, Adaptive: true, Segments: true, RefineAfterRepair: true})
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			in := testutil.BaseRequest()
			in.Mode, in.TimeLimitMS = contracts.SolveModeOptimized, 100
			in.Orders[0].ReceivedAt = testutil.At(8+i, 0)
			before, _ := json.Marshal(in)
			result, diagnostics, err := solver.SolveWithDiagnostics(context.Background(), in)
			if err != nil {
				t.Error(err)
				return
			}
			if len(result.Routes) != 1 || len(result.Routes[0].Visits) != 1 || result.Routes[0].Legs[0].StartAt.Before(in.Orders[0].ReceivedAt) || diagnostics.Estimates == 0 {
				t.Errorf("model %d lost assignments, release constraint or its diagnostics: %+v", i, result)
			}
			after, _ := json.Marshal(in)
			if string(before) != string(after) {
				t.Errorf("model %d changed shared input", i)
			}
		}(i)
	}
	group.Wait()
}

func BenchmarkRouteEstimate(b *testing.B) {
	in := testutil.BaseRequest()
	in.Orders = nil
	in.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 10
	in.Engineers[0].EquipmentStock[contracts.EquipmentRouter] = 10
	seq := make([]int, 10)
	for i := range seq {
		seq[i] = i
		order := testutil.Order(strconv.Itoa(i), "p1", int64(i), 7, 0, 30, 0)
		order.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
		in.Orders = append(in.Orders, order)
	}
	p, err := prepareRouting(in)
	if err != nil {
		b.Fatal(err)
	}
	p.prepareFastEstimate()
	for _, test := range []struct {
		name     string
		estimate func(int, []int) routeEstimate
	}{{"reference", p.referenceEstimate}, {"fast", p.fastEstimate}} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !test.estimate(0, seq).ok {
					b.Fatal("benchmark route is infeasible")
				}
			}
		})
	}
}
