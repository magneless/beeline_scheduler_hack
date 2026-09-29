//go:build vroom

package optimized

import (
	"context"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

// Check VROOM itself before any local repair can compensate for a broken native
// model or an internal process timeout. Both transport matrices must be usable
// without OSRM and stock is the current remainder, not the engineer's stock.
func TestVROOMNativeConstraintsWithoutRepair(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	in.Orders = []contracts.Order{
		testutil.Order("car", "p1", 1, 7, 0, 10, 0),
		testutil.Order("walk", "p2", 2, 7, 0, 10, 0),
		testutil.Order("unreachable", "p3", 3, 7, 0, 10, 0),
		testutil.Order("no-skill", "p1", 4, 7, 0, 10, 0),
	}
	in.Orders[0].RequiredTransport = testutil.TransportPtr(contracts.TransportCar)
	in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	in.Orders[1].RequiredTransport = testutil.TransportPtr(contracts.TransportWalk)
	in.Orders[1].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentTVBox: 1}
	in.Orders[3].RequiredSkills = []string{"missing"}
	in.Engineers = append(in.Engineers, testutil.Engineer("walker", 2))
	in.Engineers[1].Transport = contracts.TransportWalk
	in.Engineers[1].EquipmentStock[contracts.EquipmentTVBox] = 1
	in.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 1
	in.EngineerStates = append(in.EngineerStates, contracts.EngineerState{EngineerID: "walker", StartLocationID: "depot", AvailableFrom: testutil.At(6, 0), EquipmentAvailable: map[contracts.Equipment]int64{contracts.EquipmentTVBox: 1}})
	in.TravelMatrix = testutil.Matrix([]string{"depot", "p1", "p2", "p3"}, contracts.TransportCar)
	in.TravelMatrix.Profiles[contracts.TransportWalk] = testutil.Matrix(in.TravelMatrix.LocationIDs, contracts.TransportWalk).Profiles[contracts.TransportWalk]
	for _, profile := range in.TravelMatrix.Profiles {
		for from := 0; from < 3; from++ {
			profile[from][3] = contracts.TravelCell{}
		}
		profile[1][0], profile[2][0] = contracts.TravelCell{}, contracts.TravelCell{}
	}
	p := modelProblem(t, in)
	input, err := p.vroomRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	binary, err := New().executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := runVROOM(context.Background(), binary, input, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	seq, err := p.vroomSequences(input, data)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.materialize(seq)
	if err != nil {
		t.Fatal(err)
	}
	assignments := testutil.AssignedIDs(result)
	if len(assignments) != 2 || assignments["car"] != "e1" || assignments["walk"] != "walker" || len(result.Unassigned) != 2 {
		t.Fatalf("native result must satisfy model without any repair: %+v", result)
	}
}
