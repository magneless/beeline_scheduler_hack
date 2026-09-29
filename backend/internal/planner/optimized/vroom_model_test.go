package optimized

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

func modelProblem(t *testing.T, input contracts.SolveRequest) *routingProblem {
	t.Helper()
	p, err := prepareRouting(input)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVROOMModelPreservesConstraintsAndInput(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders[0].ReceivedAt = testutil.At(8, 0)
	in.Orders[0].Window.End = testutil.At(8, 5)
	in.Orders[0].RequiredTransport = testutil.TransportPtr(contracts.TransportWalk)
	in.Orders[0].RequiredSkills = []string{"walk", "repair"}
	in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1, contracts.EquipmentTVBox: 2}
	in.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 1
	in.TravelMatrix.Profiles[contracts.TransportCar][1][0] = contracts.TravelCell{}
	before, _ := json.Marshal(in)
	p := modelProblem(t, in)
	model, err := p.vroomRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	j, v := model.Jobs[0], model.Vehicles[0]
	if j.Delivery != [2]int64{1, 2} || v.Capacity != [2]int64{1, 0} {
		t.Fatalf("equipment demand / remaining stock: %+v %+v", j, v)
	}
	if j.TimeWindows[0] != [2]int64{testutil.At(8, 0).Unix() - p.origin, testutil.At(8, 5).Unix() - p.origin} {
		t.Fatal("window must constrain service start, including received_at, without subtracting service")
	}
	if j.Skills[0] == j.Skills[2] || j.Skills[2] == v.Skills[0] {
		t.Fatal("user skill walk and car/walk transport skills collided")
	}
	m := model.Matrices[contracts.TransportCar]
	if m.Durations[1][0] <= v.TimeWindow[1]-v.TimeWindow[0] || m.Durations[0][1] != 600 || m.Costs[0][1] != 1000 {
		t.Fatalf("travel matrix: %+v", m)
	}
	encoded, _ := json.Marshal(v)
	if strings.Contains(string(encoded), "end_index") || !strings.Contains(string(encoded), `"start_index":0`) {
		t.Fatalf("must encode an open route with its zero-index start: %s", encoded)
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("model preparation mutated input")
	}
}

func TestVROOMModelChecksNativeNumericBounds(t *testing.T) {
	for _, name := range []string{"distance", "fixed-cost", "horizon"} {
		t.Run(name, func(t *testing.T) {
			in := testutil.BaseRequest()
			switch name {
			case "distance":
				in.TravelMatrix.Profiles[contracts.TransportCar][0][1].DistanceM = testutil.Ptr(math.MaxUint32 + 1)
			case "fixed-cost":
				in.TravelMatrix.Profiles[contracts.TransportCar][0][1].DistanceM = testutil.Ptr(math.MaxUint32)
			case "horizon":
				in.Engineers[0].Shift.End = in.Engineers[0].Shift.Start.Add(time.Duration(math.MaxUint32) * time.Second)
			}
			if _, err := modelProblem(t, in).vroomRequest(context.Background()); err == nil {
				t.Fatal("out-of-range VROOM input accepted")
			}
		})
	}
}

func TestVROOMModelOmitsImpossibleWindowsAndRetainsOrderIDs(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders = append(in.Orders, testutil.Order("second", "p1", 2, 7, 0, 10, 0))
	in.Orders[0].ReceivedAt = testutil.At(20, 0)
	p := modelProblem(t, in)
	model, err := p.vroomRequest(context.Background())
	if err != nil || len(model.Jobs) != 1 || model.Jobs[0].ID != 2 {
		t.Fatalf("stable IDs after omitting impossible job: %+v %v", model, err)
	}
}

func TestVROOMEarlyEmergencyCandidateDoesNotNarrowOriginal(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders[0].WorkType = contracts.WorkTypeEmergency
	in.Orders[0].ReceivedAt = testutil.At(8, 0)
	p := modelProblem(t, in)
	model, err := p.vroomRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tight, changed := p.earlyEmergencyWindows(model)
	if !changed || tight.Jobs[0].TimeWindows[0][1] != testutil.At(8, 10).Unix()-p.origin {
		t.Fatalf("expected earliest feasible direct emergency arrival: %+v", tight.Jobs)
	}
	if model.Jobs[0].TimeWindows[0][1] != in.Orders[0].Window.End.Unix()-p.origin {
		t.Fatal("tight candidate mutated original window")
	}
}

func TestVROOMRejectsMalformedOrIncompleteSolutions(t *testing.T) {
	p := modelProblem(t, testutil.BaseRequest())
	input, err := p.vroomRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for name, response := range map[string]string{
		"malformed":               `no json`,
		"no code":                 `{}`,
		"engine error":            `{"code":2,"error":"invalid"}`,
		"missing job":             `{"code":0,"routes":[],"unassigned":[]}`,
		"unknown vehicle":         `{"code":0,"routes":[{"vehicle":9,"steps":[{"type":"job","id":1}]}]}`,
		"missing vehicle":         `{"code":0,"routes":[{"steps":[{"type":"job","id":1}]}]}`,
		"unknown job":             `{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"job","id":2}]}]}`,
		"duplicate job":           `{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"job","id":1},{"type":"job","id":1}]}]}`,
		"assigned and unassigned": `{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"job","id":1}]}],"unassigned":[{"type":"job","id":1}]}`,
		"duplicate vehicle":       `{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"job","id":1}]},{"vehicle":1,"steps":[]}]}`,
		"unexpected shipment":     `{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"pickup","id":1}]}]}`,
		"violation":               `{"code":0,"routes":[{"vehicle":1,"violations":[{"cause":"load"}],"steps":[{"type":"job","id":1}]}]}`,
		"step violation":          `{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"job","id":1,"violations":[{"cause":"delay"}]}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := p.vroomSequences(input, []byte(response)); err == nil {
				t.Fatal("invalid native response accepted")
			}
		})
	}
	for _, response := range []string{
		`{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"start"},{"type":"job","id":1},{"type":"end"}]}],"unassigned":[]}`,
		`{"code":0,"routes":[],"unassigned":[{"type":"job","id":1}]}`,
	} {
		if _, err := p.vroomSequences(input, []byte(response)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVROOMCandidateRechecksDepartureAfterReceipt(t *testing.T) {
	in := testutil.BaseRequest()
	in.Orders[0].ReceivedAt = testutil.At(8, 0)
	in.Orders[0].Window = contracts.Window{Start: testutil.At(8, 5), End: testutil.At(8, 5)}
	p := modelProblem(t, in)
	input, err := p.vroomRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seq, err := p.vroomSequences(input, []byte(`{"code":0,"routes":[{"vehicle":1,"steps":[{"type":"job","id":1,"arrival":1}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.materialize(p.feasiblePrefixes(seq))
	if err != nil || len(result.Routes) != 0 || len(result.Unassigned) != 1 {
		t.Fatalf("pre-receipt travel accepted or job lost: %+v %v", result, err)
	}
}
