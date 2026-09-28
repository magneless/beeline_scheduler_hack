package plans

import (
	"context"
	"reflect"
	"testing"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestRemainingGeometryKeepsUpcomingBend(t *testing.T) {
	full := []contracts.Point{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 1}, {Lat: 1, Lon: 1}}
	current := contracts.Point{Lat: 0, Lon: 0.9}
	want := []contracts.Point{current, full[1], full[2]}
	if got := remainingGeometry(full, []contracts.Point{full[0], current}); !reflect.DeepEqual(got, want) {
		t.Fatalf("remaining road shape was shortened: got %+v, want %+v", got, want)
	}
}

func TestCancellationDuringTripKeepsDriveToDestination(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	geo := standardGeo()
	geo.positionFn = func(contracts.PositionRequest) (contracts.PositionResult, error) {
		return contracts.PositionResult{Point: contracts.Point{Lat: 55.755, Lon: 37.61}, ElapsedDurationSec: 450, ElapsedDistanceM: 600, ElapsedGeometry: []contracts.Point{snapshot.Locations[0].Point, {Lat: 55.755, Lon: 37.61}}}, nil
	}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{RequestID: "cancel-in-trip", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, Event: contracts.Event{ID: "cancel-in-trip", Type: contracts.EventOrderCancelled, OccurredAt: mustTime("2026-09-17T06:07:30Z"), Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", Reason: "client_refusal"})}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Draft.Routes) != 1 || len(result.Draft.Routes[0].Visits) != 0 || len(result.Draft.Routes[0].Legs) != 2 || result.Draft.Metrics.TotalDistanceM != 1200 || result.TargetSnapshot.Orders[0].Status != contracts.OrderStatusCancelled {
		t.Fatalf("cancelled trip was redirected or canceled work was retained: %+v", result.Draft)
	}
}
