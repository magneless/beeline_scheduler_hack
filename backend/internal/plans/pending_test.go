package plans

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	geoapi "github.com/magneless/beeline_scheduler_hack/backend/internal/geo"
)

func queued(events ...contracts.Event) contracts.Event {
	return contracts.Event{ID: "queue", Type: pendingBatch, OccurredAt: events[len(events)-1].OccurredAt, Payload: contracts.EncodePayload(contracts.EventPayload{Events: events})}
}

func TestQueuedLateCompletionPreservesOtherCrews(t *testing.T) {
	snapshot, base := workStatusFixture()
	departure, start, end := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt, base.Routes[0].Visits[0].EndAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start, ExpectedEndAt: &end}
	geo, planner := standardGeo(), &fakePlanner{solveFn: solveFirstOrder}
	geo.positionFn = func(in contracts.PositionRequest) (contracts.PositionResult, error) {
		return geoapi.NewGeoService(&geoapi.DemoProvider{}).PositionAt(context.Background(), in)
	}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	event := queued(contracts.Event{ID: "finished-late", Type: contracts.EventOrderStatusChanged, OccurredAt: end.Add(time.Minute), Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: snapshot.Orders[0].ID, EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})})
	input := contracts.ReplanRequest{RequestID: "manual", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, Event: event, OptionKey: "strict"}
	preview, _, err := service.PreparePending(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Orders[0].Status != contracts.OrderStatusCompleted || geo.matrixCalls != 0 || len(planner.modes) != 0 {
		t.Fatal("recording late completion calculated routes")
	}
	result, err := service.Replan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(planner.modes) == 0 {
		t.Fatal("manual completion recalculation did not solve remaining route")
	}
	for _, route := range result.Draft.Routes {
		if route.EngineerID == "eng-2" {
			if !reflect.DeepEqual(route, base.Routes[1]) {
				t.Fatal("other crew was replanned")
			}
			return
		}
	}
	t.Fatal("other crew disappeared")
}

func TestQueuedUnavailabilityDoesNotExtendElapsedTrip(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	geo := &fakeGeo{positionFn: func(input contracts.PositionRequest) (contracts.PositionResult, error) {
		return contracts.PositionResult{Point: contracts.Point{Lat: 55.755, Lon: 37.61}, ElapsedDurationSec: int64(input.At.Sub(input.Leg.StartAt) / time.Second), ElapsedDistanceM: 600, ElapsedGeometry: []contracts.Point{{Lat: 55.75, Lon: 37.61}, {Lat: 55.755, Lon: 37.61}}}, nil
	}}
	events := queued(
		contracts.Event{ID: "disabled", Type: contracts.EventEngineerUnavailable, OccurredAt: mustTime("2026-09-17T06:07:30Z"), Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})},
		contracts.Event{ID: "cancel", Type: contracts.EventOrderCancelled, OccurredAt: mustTime("2026-09-17T06:10:00Z"), Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", Reason: "client_refusal"})},
	)
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, &fakePlanner{})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{RequestID: "manual", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, BasePlanID: base.ID, Event: events, OptionKey: "strict"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Draft.Routes) != 1 || len(result.Draft.Routes[0].Legs) != 1 || !result.Draft.Routes[0].Legs[0].EndAt.Equal(mustTime("2026-09-17T06:07:30Z")) {
		t.Fatalf("disabled crew continued travelling: %+v", result.Draft.Routes)
	}
}

func TestRemoveUnavailablePreservesOtherRoutesWithoutRouting(t *testing.T) {
	snapshot, base := workStatusFixture()
	geo, planner := &fakeGeo{}, &fakePlanner{}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	event := queued(contracts.Event{ID: "unavailable", Type: contracts.EventEngineerUnavailable, OccurredAt: mustTime("2026-09-17T05:00:00Z"), Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})})
	input := contracts.ReplanRequest{RequestID: "remove", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, BasePlanID: base.ID, Event: event, OptionKey: "remove_unavailable"}
	result, err := service.Replan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if geo.matrixCalls != 0 || geo.routesCalls != 0 || len(planner.modes) != 0 {
		t.Fatal("removal calculated routes")
	}
	if len(result.Draft.Routes) != 1 || !reflect.DeepEqual(result.Draft.Routes[0], base.Routes[1]) {
		t.Fatal("removal modified other crew")
	}
	input.OptionKey = "original"
	if _, err = service.Replan(context.Background(), input); err == nil {
		t.Fatal("original option allowed for disabled crew")
	}
}
