package plans

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

// An engineer made unavailable by the event cannot be called back as reserve
// by a different candidate for that same event.
func TestDispatchUnavailableReserveIsNotReactivated(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Engineers[0].Reserve = true
	base := testSavedPlan(snapshot)
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "reserve-unavailable", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, OptionKey: "reserve",
		Event: contracts.Event{ID: "engineer-unavailable", OccurredAt: mustTime("2026-09-17T05:00:00Z"), Type: contracts.EventEngineerUnavailable, Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetSnapshot.Engineers[0].Available {
		t.Fatal("engineer declared unavailable was reactivated as reserve")
	}
	for _, route := range result.Draft.Routes {
		if route.EngineerID == "eng-1" && len(route.Visits) > 0 {
			t.Fatal("unavailable engineer retained future assignments")
		}
	}
}

func TestDispatchPreviouslyUnavailableEngineerCannotReturnAsReserve(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Engineers[0].Reserve = true
	base := testSavedPlan(snapshot)
	geo := standardGeo()
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	first, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "unavailable-first", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "unavailable-eng-1", OccurredAt: mustTime("2026-09-17T05:00:00Z"), Type: contracts.EventEngineerUnavailable, Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	at := mustTime("2026-09-17T05:10:00Z")
	newOrder := snapshot.Orders[0]
	newOrder.ID, newOrder.SourceOrder, newOrder.ReceivedAt = "new-order", 2, at
	newOrder.WorkType, newOrder.ServiceSec = contracts.WorkTypeRepair, 600
	secondBase := contracts.Plan{ID: "plan-2", PlanDraft: first.Draft}
	secondService := mustService(t, &fakeData{snapshot: first.TargetSnapshot, plan: secondBase}, geo, planner)
	second, err := secondService.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "later-reserve-choice", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 2, BasePlanID: secondBase.ID, OptionKey: "reserve",
		Event: contracts.Event{ID: "ordinary-after-unavailable", OccurredAt: at, Type: contracts.EventOrdinaryOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &newOrder})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.TargetSnapshot.Engineers[0].Available {
		t.Fatal("an engineer declared unavailable by an earlier event returned as reserve")
	}
	for _, route := range second.Draft.Routes {
		if len(route.Visits) > 0 {
			t.Fatalf("unavailable engineer received new work: %+v", route)
		}
	}
}

func TestDispatchAutomaticEnRouteLocksOrdinaryDestination(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	at := mustTime("2026-09-17T06:07:30Z")
	geo := standardGeo()
	geo.positionFn = func(contracts.PositionRequest) (contracts.PositionResult, error) {
		return contracts.PositionResult{
			Point: contracts.Point{Lat: 55.755, Lon: 37.61}, ElapsedDurationSec: 450, ElapsedDistanceM: 600,
			ElapsedGeometry: []contracts.Point{{Lat: 55.75, Lon: 37.61}, {Lat: 55.755, Lon: 37.61}},
		}, nil
	}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, &fakePlanner{solveFn: solveFirstOrder})
	replay, err := service.replayAt(context.Background(), &snapshot, base, contracts.Event{ID: "ordinary-mid-trip", OccurredAt: at, Type: contracts.EventOrdinaryOrderAdded})
	if err != nil {
		t.Fatal(err)
	}
	if _, locked := replay.lockedOrders["order-1"]; !locked {
		t.Fatal("current destination was not locked despite scheduled departure")
	}
	if geo.positionCalls != 1 || len(replay.routes) != 1 || len(replay.routes[0].Legs) != 1 || replay.routes[0].Legs[0].DistanceM != 600 {
		t.Fatalf("current travel was not preserved: calls=%d routes=%+v", geo.positionCalls, replay.routes)
	}
}

func TestDispatchLateCompletionPreservesOtherCrew(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	second := snapshot.Orders[0]
	second.ID, second.SourceOrder = "order-2", 2
	snapshot.Orders = append(snapshot.Orders, second)
	secondEngineer := snapshot.Engineers[0]
	secondEngineer.ID, secondEngineer.SourceOrder = "eng-2", 2
	snapshot.Engineers = append(snapshot.Engineers, secondEngineer)
	otherRoute := base.Routes[0]
	otherRoute.EngineerID = "eng-2"
	otherRoute.Legs = append([]contracts.Leg(nil), otherRoute.Legs...)
	otherRoute.Legs[0].ID = "leg-2"
	otherRoute.Legs[0].StartAt = mustTime("2026-09-17T08:00:00Z")
	otherRoute.Legs[0].EndAt = mustTime("2026-09-17T08:15:00Z")
	otherRoute.Visits = append([]contracts.Visit(nil), otherRoute.Visits...)
	otherRoute.Visits[0].OrderID = "order-2"
	otherRoute.Visits[0].ArrivalAt = mustTime("2026-09-17T08:15:00Z")
	otherRoute.Visits[0].StartAt = mustTime("2026-09-17T08:15:00Z")
	otherRoute.Visits[0].EndAt = mustTime("2026-09-17T08:45:00Z")
	otherRoute.StartAt = otherRoute.Legs[0].StartAt
	base.Routes = append(base.Routes, otherRoute)
	base.Metrics.AssignedCount = 2
	base.Metrics.UsedEngineerCount = 2
	base.Metrics.TotalDistanceM = 2400
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	departure, start := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start}
	geo := standardGeo()
	planner := &fakePlanner{solveFn: func(request contracts.SolveRequest) (contracts.SolveResult, error) {
		unassigned := make([]contracts.UnassignedOrder, 0, len(request.Orders))
		for _, order := range request.Orders {
			unassigned = append(unassigned, contracts.UnassignedOrder{OrderID: order.ID, ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "Нет допустимого времени"})
		}
		return contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: unassigned, Termination: contracts.TerminationCompleted}, nil
	}}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "late-completion", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "complete-late", OccurredAt: mustTime("2026-09-17T07:45:00Z"), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range result.Draft.Routes {
		if route.EngineerID == "eng-2" {
			if !reflect.DeepEqual(route.Visits, otherRoute.Visits) || !reflect.DeepEqual(route.Legs, otherRoute.Legs) {
				t.Fatalf("unrelated crew changed after eng-1 completed late: %+v", route)
			}
			return
		}
	}
	t.Fatal("unrelated crew was removed from the plan")
}

func TestDispatchLateCompletionPreservesOtherCrewInTransit(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	second := snapshot.Orders[0]
	second.ID, second.SourceOrder = "order-2", 2
	snapshot.Orders = append(snapshot.Orders, second)
	secondEngineer := snapshot.Engineers[0]
	secondEngineer.ID, secondEngineer.SourceOrder = "eng-2", 2
	snapshot.Engineers = append(snapshot.Engineers, secondEngineer)
	otherRoute := base.Routes[0]
	otherRoute.EngineerID = "eng-2"
	otherRoute.Legs = append([]contracts.Leg(nil), otherRoute.Legs...)
	otherRoute.Legs[0].ID = "leg-2"
	otherRoute.Legs[0].StartAt = mustTime("2026-09-17T07:40:00Z")
	otherRoute.Legs[0].EndAt = mustTime("2026-09-17T07:55:00Z")
	otherRoute.Visits = append([]contracts.Visit(nil), otherRoute.Visits...)
	otherRoute.Visits[0].OrderID = "order-2"
	otherRoute.Visits[0].ArrivalAt = mustTime("2026-09-17T07:55:00Z")
	otherRoute.Visits[0].StartAt = mustTime("2026-09-17T08:00:00Z")
	otherRoute.Visits[0].EndAt = mustTime("2026-09-17T08:30:00Z")
	otherRoute.StartAt = otherRoute.Legs[0].StartAt
	base.Routes = append(base.Routes, otherRoute)
	departure, start := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start}
	geo := standardGeo()
	geo.positionFn = func(contracts.PositionRequest) (contracts.PositionResult, error) {
		return contracts.PositionResult{Point: contracts.Point{Lat: 55.755, Lon: 37.61}, ElapsedDurationSec: 300, ElapsedDistanceM: 400,
			ElapsedGeometry: []contracts.Point{snapshot.Locations[0].Point, {Lat: 55.755, Lon: 37.61}}}, nil
	}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "late-completion-in-transit", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "complete-late-in-transit", OccurredAt: mustTime("2026-09-17T07:45:00Z"), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range result.Draft.Routes {
		if route.EngineerID != "eng-2" {
			continue
		}
		for _, visit := range route.Visits {
			if visit.OrderID == "order-2" && visit.StartAt.Equal(otherRoute.Visits[0].StartAt) {
				return
			}
		}
		t.Fatalf("other crew's current destination disappeared: %+v", route)
	}
	t.Fatal("other crew's route disappeared")
}

func TestDispatchLateCompletionPreservesOtherCrewsFutureAfterCompletedVisit(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	second := snapshot.Orders[0]
	second.ID, second.SourceOrder = "order-2", 2
	third := snapshot.Orders[0]
	third.ID, third.SourceOrder = "order-3", 3
	snapshot.Orders = append(snapshot.Orders, second, third)
	secondEngineer := snapshot.Engineers[0]
	secondEngineer.ID, secondEngineer.SourceOrder = "eng-2", 2
	snapshot.Engineers = append(snapshot.Engineers, secondEngineer)
	otherRoute := base.Routes[0]
	otherRoute.EngineerID = "eng-2"
	otherRoute.Legs = append([]contracts.Leg(nil), otherRoute.Legs...)
	otherRoute.Legs[0].ID = "leg-2"
	otherRoute.Visits = append([]contracts.Visit(nil), otherRoute.Visits...)
	otherRoute.Visits[0].OrderID = "order-2"
	otherRoute.Legs = append(otherRoute.Legs, contracts.Leg{ID: "leg-3", FromLocationID: "loc-1", ToLocationID: "loc-1", StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:00:00Z"), DistanceM: 0, GeoContextID: "geo-1", Geometry: []contracts.Point{snapshot.Locations[1].Point, snapshot.Locations[1].Point}})
	otherRoute.Visits = append(otherRoute.Visits, contracts.Visit{OrderID: "order-3", ArrivalAt: mustTime("2026-09-17T08:00:00Z"), StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:30:00Z")})
	base.Routes = append(base.Routes, otherRoute)
	departure, start := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start}
	finish := otherRoute.Visits[0].EndAt
	snapshot.Orders[1].Status = contracts.OrderStatusCompleted
	snapshot.Orders[1].Execution = &contracts.OrderExecution{EngineerID: "eng-2", DepartedAt: &departure, StartedAt: &start, FinishedAt: &finish}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "late-completion-other-future", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "complete-late-other-future", OccurredAt: mustTime("2026-09-17T07:45:00Z"), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range result.Draft.Routes {
		if route.EngineerID == "eng-2" {
			for _, visit := range route.Visits {
				if visit.OrderID == "order-3" && visit.StartAt.Equal(otherRoute.Visits[1].StartAt) {
					return
				}
			}
		}
	}
	t.Fatal("unrelated crew's future visit disappeared after its previous visit completed")
}

func TestDispatchEarlyCompletionKeepsNextDeparture(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: "loc-2", Address: "Москва, следующий адрес", Point: contracts.Point{Lat: 55.77, Lon: 37.62}})
	next := snapshot.Orders[0]
	next.ID, next.LocationID, next.SourceOrder = "order-2", "loc-2", 2
	snapshot.Orders = append(snapshot.Orders, next)
	base.Routes[0].Legs = append(base.Routes[0].Legs, contracts.Leg{
		ID: "leg-2", FromLocationID: "loc-1", ToLocationID: "loc-2", StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:15:00Z"), DistanceM: 1200, GeoContextID: "geo-1",
		Geometry: []contracts.Point{snapshot.Locations[1].Point, snapshot.Locations[2].Point},
	})
	base.Routes[0].Visits = append(base.Routes[0].Visits, contracts.Visit{OrderID: next.ID, ArrivalAt: mustTime("2026-09-17T08:15:00Z"), StartAt: mustTime("2026-09-17T08:15:00Z"), EndAt: mustTime("2026-09-17T08:45:00Z")})
	departure, start := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start}
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), planner)
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "early-completion", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "complete-early", OccurredAt: mustTime("2026-09-17T07:20:00Z"), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planner.modes) != 0 {
		t.Fatalf("early completion unexpectedly ran the solver: %v", planner.modes)
	}
	for _, route := range result.Draft.Routes {
		for i, visit := range route.Visits {
			if visit.OrderID == next.ID {
				if visit.StartAt != base.Routes[0].Visits[1].StartAt || route.Legs[i].StartAt != base.Routes[0].Legs[1].StartAt {
					t.Fatalf("early completion advanced next departure: visit=%+v leg=%+v", visit, route.Legs[i])
				}
				return
			}
		}
	}
	t.Fatal("future visit disappeared after early completion")
}

func TestDispatchCancellationDoesNotInterruptStartedWork(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	departure, start := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
	_, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "cancel-started", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "cancel-active", OccurredAt: mustTime("2026-09-17T07:10:00Z"), Type: contracts.EventOrderCancelled, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", Reason: "client_refusal"})},
	})
	requireContractCode(t, err, contracts.ErrorEventConflict)
}

func TestDispatchWorkStartDoesNotRunSolver(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: "loc-2", Address: "Москва, следующий адрес", Point: contracts.Point{Lat: 55.77, Lon: 37.62}})
	next := snapshot.Orders[0]
	next.ID, next.LocationID, next.SourceOrder = "order-2", "loc-2", 2
	snapshot.Orders = append(snapshot.Orders, next)
	base.Routes[0].Legs = append(base.Routes[0].Legs, contracts.Leg{ID: "leg-2", FromLocationID: "loc-1", ToLocationID: "loc-2", StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:15:00Z"), DistanceM: 1200, GeoContextID: "geo-1", Geometry: []contracts.Point{snapshot.Locations[1].Point, snapshot.Locations[2].Point}})
	base.Routes[0].Visits = append(base.Routes[0].Visits, contracts.Visit{OrderID: next.ID, ArrivalAt: mustTime("2026-09-17T08:15:00Z"), StartAt: mustTime("2026-09-17T08:15:00Z"), EndAt: mustTime("2026-09-17T08:45:00Z")})
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), planner)
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "work-start", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "start-order-1", OccurredAt: base.Routes[0].Visits[0].StartAt, Type: contracts.EventOrderStatusChanged,
			Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusInProgress})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planner.modes) != 0 {
		t.Fatalf("starting work unexpectedly ran the solver: %v", planner.modes)
	}
	if result.TargetSnapshot.Orders[0].Status != contracts.OrderStatusInProgress || result.TargetSnapshot.Orders[0].Execution == nil || result.TargetSnapshot.Orders[0].Execution.StartedAt == nil {
		t.Fatalf("work start was not recorded: %+v", result.TargetSnapshot.Orders[0])
	}
	for _, route := range result.Draft.Routes {
		for _, visit := range route.Visits {
			if visit.OrderID == next.ID && visit.StartAt.Equal(base.Routes[0].Visits[1].StartAt) {
				return
			}
		}
	}
	t.Fatal("work start without expected end removed the later visit")
}

func TestDispatchPreviouslyUnassignedNeverReenters(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	previous := snapshot.Orders[0]
	previous.ID, previous.SourceOrder = "previously-unassigned", 2
	snapshot.Orders = append(snapshot.Orders, previous)
	base.Unassigned = append(base.Unassigned, contracts.UnassignedOrder{OrderID: previous.ID, ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "Перенос вне сервиса"})
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: func(request contracts.SolveRequest) (contracts.SolveResult, error) {
		for _, order := range request.Orders {
			if order.ID == previous.ID {
				t.Fatal("previously unassigned order was submitted to solver")
			}
		}
		return solveFirstOrder(request)
	}})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "exclude-previous", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "engineer-unavailable", OccurredAt: mustTime("2026-09-17T05:00:00Z"), Type: contracts.EventEngineerUnavailable, Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range result.Draft.Routes {
		for _, visit := range route.Visits {
			if visit.OrderID == previous.ID {
				t.Fatal("previously unassigned order entered the plan")
			}
		}
	}
	found := false
	for _, item := range result.Draft.Unassigned {
		found = found || item.OrderID == previous.ID
	}
	if !found {
		t.Fatal("previously unassigned order disappeared from dispatcher list")
	}
}

func TestDispatchInitialOptionsLateEmergencyKeepsOriginalWindow(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Orders[0].WorkType = contracts.WorkTypeEmergency
	snapshot.Orders[0].Priority = contracts.PriorityUrgent
	snapshot.Orders[0].ServiceSec = 4800
	snapshot.Orders[0].Window = contracts.Window{Start: mustTime("2026-09-17T06:00:00Z"), End: mustTime("2026-09-17T06:05:00Z")}
	snapshot.Orders[0].ReceivedAt = mustTime("2026-09-17T05:00:00Z")
	planner := &fakePlanner{solveFn: func(request contracts.SolveRequest) (contracts.SolveResult, error) {
		if request.Orders[0].Window.End.Equal(snapshot.Orders[0].Window.End) {
			return contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{{OrderID: "order-1", ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "Окно истекло до прибытия"}}, Termination: contracts.TerminationCompleted}, nil
		}
		return solveFirstOrder(request)
	}}
	service := mustService(t, &fakeData{snapshot: snapshot}, standardGeo(), planner)
	options, err := service.BuildOptions(context.Background(), contracts.BuildPlanRequest{RequestID: "initial-options", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].Key != "strict" || options[1].Key != "late_emergency" {
		t.Fatalf("unexpected initial variants: %+v", options)
	}
	if len(options[0].Result.Draft.Routes) != 0 || len(options[1].Result.Draft.Routes) != 1 {
		t.Fatalf("late option did not recover strict-unassigned emergency: %+v", options)
	}
	late := options[1]
	if !reflect.DeepEqual(late.Result.TargetSnapshot.Orders[0].Window, snapshot.Orders[0].Window) {
		t.Fatal("late option changed the customer's original emergency window")
	}
	if len(late.Lateness) != 1 || late.Lateness[0].OrderID != "order-1" || late.Lateness[0].LateSec != int64((10*time.Minute)/time.Second) {
		t.Fatalf("late option did not report planned delay: %+v", late.Lateness)
	}
}

func TestDispatchReserveOptionActivatesOnlyUsedReserve(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	used := snapshot.Engineers[0]
	used.ID, used.SourceOrder, used.Reserve, used.Available = "eng-2", 2, true, false
	unused := used
	unused.ID, unused.SourceOrder = "eng-3", 3
	snapshot.Engineers = append(snapshot.Engineers, used, unused)
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
	options, err := service.ReplanOptions(context.Background(), contracts.ReplanRequest{
		RequestID: "reserve-options", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "unavailable-eng-1", OccurredAt: mustTime("2026-09-17T05:00:00Z"), Type: contracts.EventEngineerUnavailable, Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 3 || options[0].Key != "strict" || options[1].Key != "remove_unavailable" || options[2].Key != "reserve" {
		t.Fatalf("unexpected nonurgent choices: %+v", options)
	}
	reserve := options[2]
	if len(reserve.ReserveEngineerIDs) != 1 || reserve.ReserveEngineerIDs[0] != "eng-2" || !reserve.Result.TargetSnapshot.Engineers[1].Available || reserve.Result.TargetSnapshot.Engineers[2].Available {
		t.Fatalf("only assigned reserve engineer should move into working crew: %+v %+v", reserve.ReserveEngineerIDs, reserve.Result.TargetSnapshot.Engineers)
	}
	if len(reserve.Result.Draft.Routes) != 1 || reserve.Result.Draft.Routes[0].EngineerID != "eng-2" {
		t.Fatalf("reserve did not take displaced work: %+v", reserve.Result.Draft.Routes)
	}
}

func TestDispatchOriginalChoiceLeavesOrdinaryAdditionUnassigned(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	at := mustTime("2026-09-17T05:00:00Z")
	newOrder := snapshot.Orders[0]
	newOrder.ID, newOrder.SourceOrder, newOrder.ReceivedAt = "new-order", 2, at
	newOrder.WorkType, newOrder.ServiceSec = contracts.WorkTypeRepair, 600
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "keep-original", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, OptionKey: "original",
		Event: contracts.Event{ID: "ordinary-addition", OccurredAt: at, Type: contracts.EventOrdinaryOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &newOrder})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Draft.Unassigned) != 1 || result.Draft.Unassigned[0].OrderID != newOrder.ID || len(result.Draft.Routes) != 1 || !reflect.DeepEqual(result.Draft.Routes[0].Visits, base.Routes[0].Visits) {
		t.Fatalf("original choice did not preserve existing appointment and surface new order: %+v", result.Draft)
	}
}

func TestDispatchOrdinaryOptionPreservesCurrentTrip(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	at := mustTime("2026-09-17T06:07:30Z")
	newOrder := snapshot.Orders[0]
	newOrder.ID, newOrder.SourceOrder, newOrder.ReceivedAt = "new-order", 2, at
	newOrder.WorkType, newOrder.ServiceSec = contracts.WorkTypeRepair, 600
	newOrder.Window = contracts.Window{Start: at, End: mustTime("2026-09-17T09:00:00Z")}
	geo := standardGeo()
	geo.positionFn = func(contracts.PositionRequest) (contracts.PositionResult, error) {
		return contracts.PositionResult{Point: contracts.Point{Lat: 55.755, Lon: 37.61}, ElapsedDurationSec: 450, ElapsedDistanceM: 600,
			ElapsedGeometry: []contracts.Point{snapshot.Locations[0].Point, {Lat: 55.755, Lon: 37.61}}}, nil
	}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "ordinary-enroute", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, OptionKey: "strict",
		Event: contracts.Event{ID: "new-while-enroute", OccurredAt: at, Type: contracts.EventOrdinaryOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &newOrder})},
	})
	if err != nil {
		t.Fatal(err)
	}
	var original, incoming *contracts.Visit
	for _, route := range result.Draft.Routes {
		for i := range route.Visits {
			visit := &route.Visits[i]
			if visit.OrderID == "order-1" {
				original = visit
			}
			if visit.OrderID == newOrder.ID {
				incoming = visit
			}
		}
	}
	if original == nil || incoming == nil || !original.StartAt.Equal(base.Routes[0].Visits[0].StartAt) || incoming.StartAt.Before(original.EndAt) {
		t.Fatalf("ordinary option redirected or overlapped current trip: %+v", result.Draft.Routes)
	}
}

func TestDispatchRepeatedEventsKeepCurrentTripAndElapsedDistance(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	firstAt := mustTime("2026-09-17T06:05:00Z")
	newOrder := snapshot.Orders[0]
	newOrder.ID, newOrder.SourceOrder, newOrder.ReceivedAt = "new-order", 2, firstAt
	newOrder.WorkType, newOrder.ServiceSec = contracts.WorkTypeRepair, 600
	newOrder.Window = contracts.Window{Start: firstAt, End: mustTime("2026-09-17T09:00:00Z")}
	geo := standardGeo()
	geo.positionFn = func(input contracts.PositionRequest) (contracts.PositionResult, error) {
		leg := input.Leg
		elapsed := int64(input.At.Sub(leg.StartAt) / time.Second)
		total := int64(leg.EndAt.Sub(leg.StartAt) / time.Second)
		fraction := float64(elapsed) / float64(total)
		from, to := leg.Geometry[0], leg.Geometry[len(leg.Geometry)-1]
		point := contracts.Point{Lat: from.Lat + fraction*(to.Lat-from.Lat), Lon: from.Lon + fraction*(to.Lon-from.Lon)}
		return contracts.PositionResult{Point: point, ElapsedDurationSec: elapsed, ElapsedDistanceM: leg.DistanceM * elapsed / total,
			ElapsedGeometry: []contracts.Point{from, point}}, nil
	}
	planner := &fakePlanner{solveFn: solveFirstOrder}
	firstService := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	first, err := firstService.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "first-mid-trip", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, OptionKey: "strict",
		Event: contracts.Event{ID: "new-mid-trip", OccurredAt: firstAt, Type: contracts.EventOrdinaryOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &newOrder})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Draft.Metrics.TotalDistanceM != 1200 {
		t.Fatalf("first partial-trip distance changed: %d", first.Draft.Metrics.TotalDistanceM)
	}
	secondBase := contracts.Plan{ID: "plan-2", PlanDraft: first.Draft}
	secondService := mustService(t, &fakeData{snapshot: first.TargetSnapshot, plan: secondBase}, geo, planner)
	second, err := secondService.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "second-mid-trip", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 2, BasePlanID: secondBase.ID, OptionKey: "strict",
		Event: contracts.Event{ID: "cancel-new-mid-trip", OccurredAt: mustTime("2026-09-17T06:10:00Z"), Type: contracts.EventOrderCancelled, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: newOrder.ID, Reason: "client_refusal"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Draft.Metrics.TotalDistanceM != 1200 {
		t.Fatalf("repeated split double counted or lost distance: %d", second.Draft.Metrics.TotalDistanceM)
	}
	var visitFound bool
	visitCount := 0
	for _, route := range second.Draft.Routes {
		for _, visit := range route.Visits {
			if visit.OrderID == "order-1" {
				visitCount++
				visitFound = visit.StartAt.Equal(base.Routes[0].Visits[0].StartAt)
			}
		}
	}
	if !visitFound || visitCount != 1 {
		t.Fatalf("second event changed or lost current destination: %+v", second.Draft.Routes)
	}
	if len(first.Draft.Routes[0].Legs) == 0 || len(second.Draft.Routes[0].Legs) == 0 || !reflect.DeepEqual(first.Draft.Routes[0].Legs[0], second.Draft.Routes[0].Legs[0]) {
		t.Fatal("second event lost the already traveled segment")
	}
}

func TestDispatchRemainingGeometryRetainsNextBend(t *testing.T) {
	a := contracts.Point{Lat: 0, Lon: 0}
	b := contracts.Point{Lat: 0, Lon: 1}
	c := contracts.Point{Lat: 1, Lon: 1}
	d := contracts.Point{Lat: 1, Lon: 2}
	current := contracts.Point{Lat: 0.6, Lon: 1}
	remaining := remainingGeometry([]contracts.Point{a, b, c, d}, []contracts.Point{a, b, current})
	if !reflect.DeepEqual(remaining, []contracts.Point{current, c, d}) {
		t.Fatalf("remaining geometry skipped route bend: %+v", remaining)
	}
}

func TestDispatchUrgentRedirectionUpdatesCurrentTripStatus(t *testing.T) {
	for _, tc := range []struct {
		name                string
		addSecondEngineer   bool
		wantOriginalEnRoute bool
	}{
		{name: "emergency inserted ahead on same engineer", wantOriginalEnRoute: false},
		{name: "emergency handled by another engineer", addSecondEngineer: true, wantOriginalEnRoute: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := testSnapshot()
			snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: "loc-2", Address: "Москва, авария", Point: contracts.Point{Lat: 55.755, Lon: 37.61}})
			if tc.addSecondEngineer {
				secondEngineer := snapshot.Engineers[0]
				secondEngineer.ID, secondEngineer.SourceOrder = "eng-2", 2
				snapshot.Engineers = append(snapshot.Engineers, secondEngineer)
			}
			base := testSavedPlan(snapshot)
			at := mustTime("2026-09-17T06:05:00Z")
			emergency := snapshot.Orders[0]
			emergency.ID, emergency.LocationID, emergency.SourceOrder = "emergency", "loc-2", 2
			emergency.WorkType, emergency.Priority, emergency.ServiceSec = contracts.WorkTypeEmergency, contracts.PriorityUrgent, 4800
			emergency.ReceivedAt = at
			emergency.Window = contracts.Window{Start: at, End: mustTime("2026-09-17T07:30:00Z")}
			geo := standardGeo()
			geo.positionFn = func(contracts.PositionRequest) (contracts.PositionResult, error) {
				return contracts.PositionResult{Point: snapshot.Locations[2].Point, ElapsedDurationSec: 300, ElapsedDistanceM: 400,
					ElapsedGeometry: []contracts.Point{snapshot.Locations[0].Point, snapshot.Locations[2].Point}}, nil
			}
			planner := &fakePlanner{solveFn: func(request contracts.SolveRequest) (contracts.SolveResult, error) {
				orders := orderMap(request.Orders)
				engineers := engineerMap(request.Engineers)
				states := map[string]contracts.EngineerState{}
				for _, state := range request.EngineerStates {
					states[state.EngineerID] = state
				}
				makeRoute := func(engineerID string, ids ...string) contracts.Route {
					state := states[engineerID]
					route := contracts.Route{EngineerID: engineerID, StartLocationID: state.StartLocationID, StartAt: state.AvailableFrom, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}}
					location, available := state.StartLocationID, state.AvailableFrom
					for i, id := range ids {
						order := orders[id]
						cell, err := matrixCell(request.TravelMatrix, engineers[engineerID].Transport, location, order.LocationID)
						if err != nil {
							t.Fatal(err)
						}
						arrival := available.Add(time.Duration(*cell.DurationSec) * time.Second)
						start := arrival
						if order.Window.Start.After(start) {
							start = order.Window.Start
						}
						route.Legs = append(route.Legs, contracts.Leg{ID: engineerID + "-leg-" + id, FromLocationID: location, ToLocationID: order.LocationID, StartAt: available, EndAt: arrival, DistanceM: *cell.DistanceM, GeoContextID: request.TravelMatrix.GeoContextID})
						route.Visits = append(route.Visits, contracts.Visit{OrderID: id, ArrivalAt: arrival, StartAt: start, EndAt: start.Add(time.Duration(order.ServiceSec) * time.Second)})
						location, available = order.LocationID, route.Visits[i].EndAt
					}
					return route
				}
				var routes []contracts.Route
				if tc.addSecondEngineer {
					routes = []contracts.Route{makeRoute("eng-1", "order-1"), makeRoute("eng-2", "emergency")}
				} else {
					routes = []contracts.Route{makeRoute("eng-1", "emergency", "order-1")}
				}
				return contracts.SolveResult{Routes: routes, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}, nil
			}}
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
			result, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: "urgent-redirection", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, OptionKey: "strict",
				Event: contracts.Event{ID: "urgent-mid-trip", OccurredAt: at, Type: contracts.EventUrgentOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &emergency})},
			})
			if err != nil {
				t.Fatal(err)
			}
			original := result.TargetSnapshot.Orders[0]
			if tc.wantOriginalEnRoute {
				if original.Status != contracts.OrderStatusEnRoute || original.Execution == nil {
					t.Fatalf("retained destination lost in-transit status: %+v", original)
				}
			} else if original.Status != contracts.OrderStatusActive || original.Execution != nil {
				t.Fatalf("redirected original destination kept stale in-transit status: %+v", original)
			}
		})
	}
}

func TestDispatchNoLateEligibleEmergencyDoesNotRunAnotherSearch(t *testing.T) {
	for _, emergency := range []bool{false, true} {
		snapshot := testSnapshot()
		if emergency {
			snapshot.Orders[0].WorkType = contracts.WorkTypeEmergency
			snapshot.Orders[0].Priority = contracts.PriorityUrgent
		}
		calls := 0
		service := mustService(t, &fakeData{snapshot: snapshot}, standardGeo(), &fakePlanner{solveFn: func(in contracts.SolveRequest) (contracts.SolveResult, error) { calls++; return solveFirstOrder(in) }})
		options, err := service.BuildOptions(context.Background(), contracts.BuildPlanRequest{RequestID: "same-conditions", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("unchanged conditions triggered extra search: %d calls", calls)
		}
		if options[1].IdenticalTo == nil || *options[1].IdenticalTo != "strict" || !reflect.DeepEqual(options[0].Result.Draft.Routes, options[1].Result.Draft.Routes) {
			t.Fatal("identical conditions produced different routes")
		}
	}
}
