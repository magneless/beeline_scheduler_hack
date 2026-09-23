package plans

import (
	"context"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestElapsedPlanIsNotExecution(t *testing.T) {
	example := loadBackendExample(t)
	var snapshot contracts.Snapshot
	var base contracts.Plan
	decodeExample(t, example, "snapshot", &snapshot)
	decodeExample(t, example, "saved_plan", &base)
	service := mustService(t, &fakeData{}, &fakeGeo{}, &fakePlanner{})
	at := base.Routes[0].Visits[0].EndAt.Add(time.Hour)
	replay, err := service.replayAt(context.Background(), &snapshot, base, contracts.Event{ID: "later", OccurredAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if len(replay.routes) != 0 || len(replay.lockedOrders) != 0 {
		t.Fatal("elapsed planned timestamps must not establish execution facts")
	}
	for _, engineer := range snapshot.Engineers {
		if replay.states[engineer.ID].StartLocationID != snapshot.OfficeLocationID {
			t.Fatal("engineer moved without a departure fact")
		}
	}
}

func TestBuildWithNoAvailableEngineers(t *testing.T) {
	example := loadBackendExample(t)
	var snapshot contracts.Snapshot
	var request contracts.BuildPlanRequest
	decodeExample(t, example, "snapshot", &snapshot)
	decodeExample(t, example, "build_request", &request)
	for i := range snapshot.Engineers {
		snapshot.Engineers[i].Available = false
	}
	service := mustService(t, &fakeData{snapshot: snapshot}, &fakeGeo{}, &fakePlanner{})
	result, err := service.Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Draft.Routes) != 0 || len(result.Draft.Unassigned) != len(snapshot.Orders) {
		t.Fatal("all orders should remain unassigned")
	}
}
