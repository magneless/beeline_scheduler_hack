package plans

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestWorkStartChecksPreviousAppointmentEvenAtSameAddress(t *testing.T) {
	for _, status := range []contracts.OrderStatus{contracts.OrderStatusActive, contracts.OrderStatusSent, contracts.OrderStatusInProgress} {
		t.Run(string(status), func(t *testing.T) {
			snapshot, base := workStatusFixture()
			snapshot.Orders[0].Status = status
			if status != contracts.OrderStatusActive {
				snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1"}
				if status == contracts.OrderStatusInProgress {
					snapshot.Orders[0].Execution.StartedAt = ptr(base.Routes[0].Visits[0].StartAt)
				}
			}
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{})
			_, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: "next-unfinished", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
				Event: contracts.Event{ID: "next-unfinished", OccurredAt: base.Routes[0].Visits[1].StartAt, Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "next", EngineerID: "eng-1", Status: contracts.OrderStatusInProgress})},
			})
			var conflict *contracts.ContractError
			if !errors.As(err, &conflict) || conflict.Message != "previous work must be completed before starting next" || conflict.Details["previous_order_id"] != "order-1" {
				t.Fatalf("expected actionable preceding-work conflict, got %v", err)
			}
		})
	}
}

func TestWorkStartAtPrecisePlannedTime(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	start := mustTime("2026-09-17T07:00:03Z")
	base.Routes[0].Legs[0].EndAt = start
	base.Routes[0].Visits[0] = contracts.Visit{OrderID: "order-1", ArrivalAt: start, StartAt: start, EndAt: start.Add(30 * time.Minute)}
	geo, planner := &fakeGeo{}, &fakePlanner{}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	request := contracts.ReplanRequest{
		RequestID: "start-with-seconds", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "start-with-seconds", OccurredAt: start, Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusInProgress, ExpectedEndAt: ptr(base.Routes[0].Visits[0].EndAt)})},
	}
	result, err := service.Replan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.TargetSnapshot.Orders[0].Execution.StartedAt.Equal(start) || !reflect.DeepEqual(result.Draft.Routes, base.Routes) {
		t.Fatal("precise planned start changed the accepted route")
	}
	request.Event.OccurredAt = start.Truncate(time.Minute)
	_, err = service.Replan(context.Background(), request)
	requireContractCode(t, err, contracts.ErrorEventConflict)
}

func workStatusFixture() (contracts.Snapshot, contracts.Plan) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	snapshot.Engineers[0].EquipmentStock = map[contracts.Equipment]int64{contracts.EquipmentRouter: 3}
	snapshot.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
	next := snapshot.Orders[0]
	next.ID, next.SourceOrder = "next", 2
	snapshot.Orders = append(snapshot.Orders, next)
	base.Routes[0].Legs = append(base.Routes[0].Legs, contracts.Leg{
		ID: "next-leg", FromLocationID: "loc-1", ToLocationID: "loc-1", StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:00:00Z"), GeoContextID: "geo-1",
		Geometry: []contracts.Point{snapshot.Locations[1].Point, snapshot.Locations[1].Point},
	})
	base.Routes[0].Visits = append(base.Routes[0].Visits, contracts.Visit{OrderID: next.ID, ArrivalAt: mustTime("2026-09-17T08:00:00Z"), StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:30:00Z")})
	otherEngineer := snapshot.Engineers[0]
	otherEngineer.ID, otherEngineer.SourceOrder = "eng-2", 2
	snapshot.Engineers = append(snapshot.Engineers, otherEngineer)
	otherOrder := snapshot.Orders[0]
	otherOrder.ID, otherOrder.SourceOrder = "other-crew-order", 3
	snapshot.Orders = append(snapshot.Orders, otherOrder)
	other := clonePlanRoutes(base.Routes[:1])[0]
	other.EngineerID = otherEngineer.ID
	other.StartAt = mustTime("2026-09-17T06:55:00Z")
	other.Legs = other.Legs[:1]
	other.Legs[0].ID = "other-leg"
	other.Legs[0].StartAt, other.Legs[0].EndAt = other.StartAt, mustTime("2026-09-17T07:35:00Z")
	other.Visits = []contracts.Visit{{OrderID: otherOrder.ID, ArrivalAt: other.Legs[0].EndAt, StartAt: other.Legs[0].EndAt, EndAt: mustTime("2026-09-17T08:05:00Z")}}
	base.Routes = append(base.Routes, other)
	unassigned := snapshot.Orders[0]
	unassigned.ID, unassigned.SourceOrder = "unassigned", 4
	snapshot.Orders = append(snapshot.Orders, unassigned)
	base.Unassigned = []contracts.UnassignedOrder{{OrderID: unassigned.ID, ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "Перенос вне сервиса"}}
	base.Metrics = calculateMetrics(base.Routes, base.Unassigned)
	return snapshot, base
}

func TestWorkStatusPreservesScheduleWithoutGeoOrSolver(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     contracts.OrderStatus
		started    bool
		at         string
		expectedAt string
	}{
		{"start_on_time", contracts.OrderStatusInProgress, false, "07:00:00", "07:30:00"},
		{"start_without_estimate", contracts.OrderStatusInProgress, false, "07:00:00", ""},
		{"start_shorter_work", contracts.OrderStatusInProgress, false, "07:00:00", "07:20:00"},
		{"late_start", contracts.OrderStatusInProgress, false, "07:20:00", "07:50:00"},
		{"start_after_planned_end", contracts.OrderStatusInProgress, false, "07:40:00", "08:10:00"},
		{"start_after_next_departure", contracts.OrderStatusInProgress, false, "08:10:00", "08:40:00"},
		{"late_start_without_estimate", contracts.OrderStatusInProgress, false, "07:40:00", ""},
		{"same_estimate", contracts.OrderStatusInProgress, true, "07:05:00", "07:30:00"},
		{"earlier_estimate", contracts.OrderStatusInProgress, true, "07:05:00", "07:20:00"},
		{"later_estimate", contracts.OrderStatusInProgress, true, "07:05:00", "08:20:00"},
		{"update_after_planned_end", contracts.OrderStatusInProgress, true, "07:40:00", "08:20:00"},
		{"complete_on_time", contracts.OrderStatusCompleted, true, "07:30:00", ""},
		{"complete_early", contracts.OrderStatusCompleted, true, "07:20:00", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, base := workStatusFixture()
			if tc.started {
				departure, start, end := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt, base.Routes[0].Visits[0].EndAt
				snapshot.Orders[0].Status = contracts.OrderStatusInProgress
				snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start, ExpectedEndAt: &end}
				base.AsOf = start
			}
			payload := contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: tc.status}
			if tc.expectedAt != "" {
				payload.ExpectedEndAt = ptr(mustTime("2026-09-17T" + tc.expectedAt + "Z"))
			}
			at := mustTime("2026-09-17T" + tc.at + "Z")
			geo, planner := &fakeGeo{}, &fakePlanner{}
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
			before := clonePlanRoutes(base.Routes)
			result, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: tc.name, ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
				Event: contracts.Event{ID: tc.name, OccurredAt: at, Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(payload)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if geo.matrixCalls != 0 || geo.routesCalls != 0 || geo.positionCalls != 0 || len(planner.modes) != 0 {
				t.Fatalf("recording work invoked routing: geo=%+v modes=%v", geo, planner.modes)
			}
			expectedEnd := base.Routes[0].Visits[0].EndAt
			if tc.status == contracts.OrderStatusCompleted {
				expectedEnd = at
			}
			expected := clonePlanRoutes(before)
			expected[0].Visits[0].EndAt = expectedEnd
			if !reflect.DeepEqual(result.Draft.Routes, expected) || !reflect.DeepEqual(base.Routes, before) {
				t.Fatalf("another visit, departure, geometry, or base plan changed: %+v", result.Draft.Routes)
			}
			if !reflect.DeepEqual(result.Draft.Unassigned, base.Unassigned) || !reflect.DeepEqual(result.TargetSnapshot.Locations, snapshot.Locations) || !reflect.DeepEqual(result.TargetSnapshot.Orders[1:], snapshot.Orders[1:]) {
				t.Fatal("unrelated orders, reasons, or locations changed")
			}
			if result.TargetSnapshot.Orders[0].Status != tc.status || result.TargetSnapshot.Revision != snapshot.Revision+1 || !result.Draft.AsOf.Equal(at) || result.Draft.BasePlanID == nil || *result.Draft.BasePlanID != base.ID || result.AppliedEvent == nil {
				t.Fatal("execution or revision was not recorded")
			}
			if tc.status == contracts.OrderStatusInProgress {
				execution := result.TargetSnapshot.Orders[0].Execution
				startedAt := at
				if tc.started {
					startedAt = *snapshot.Orders[0].Execution.StartedAt
				}
				if execution == nil || execution.StartedAt == nil || !execution.StartedAt.Equal(startedAt) || !reflect.DeepEqual(execution.ExpectedEndAt, payload.ExpectedEndAt) {
					t.Fatal("actual start or forecast was lost while preserving the schedule")
				}
			}
			if result.Draft.EquipmentRemaining["eng-1"][contracts.EquipmentRouter] != 2 {
				t.Fatal("equipment should be consumed exactly once")
			}
			if tc.status == contracts.OrderStatusCompleted && (result.Draft.Metrics.CompletedCount != 1 || !reflect.DeepEqual(result.Draft.CompletedOrderIDs, []string{"order-1"})) {
				t.Fatal("completion counters were not updated")
			}
		})
	}
}

func TestLateStartThenCompletionUsesAcceptedEnd(t *testing.T) {
	for _, tc := range []struct {
		name, start, finish string
		replan              bool
	}{
		{"early_finish", "07:20:00", "07:25:00", false},
		{"on_time_finish", "07:20:00", "07:30:00", false},
		{"late_finish_before_forecast", "07:20:00", "07:40:00", true},
		{"late_finish_at_forecast", "07:20:00", "07:50:00", true},
		{"start_after_next_departure", "08:10:00", "08:20:00", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, base := workStatusFixture()
			data := &fakeData{snapshot: snapshot, plan: base}
			geo, planner := &fakeGeo{}, &fakePlanner{}
			service := mustService(t, data, geo, planner)
			start := mustTime("2026-09-17T" + tc.start + "Z")
			forecast := start.Add(30 * time.Minute)
			started, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: "late-start", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
				Event: contracts.Event{ID: "late-start", OccurredAt: start, Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusInProgress, ExpectedEndAt: &forecast})},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(started.Draft.Routes, base.Routes) || geo.matrixCalls != 0 || geo.routesCalls != 0 || geo.positionCalls != 0 || len(planner.modes) != 0 {
				t.Fatal("late start must record execution without routing or schedule changes")
			}
			// Accept the start result, then complete against that saved revision.
			data.snapshot = started.TargetSnapshot
			data.plan = contracts.Plan{ID: "plan-after-start", PlanDraft: started.Draft}
			if tc.replan {
				geo = standardGeo()
				planner = &fakePlanner{solveFn: func(input contracts.SolveRequest) (contracts.SolveResult, error) {
					if len(input.Engineers) != 1 || input.Engineers[0].ID != "eng-1" || len(input.Orders) != 1 || input.Orders[0].ID != "next" {
						t.Fatal("completion must only replan this crew's remaining assigned work")
					}
					return solveFirstOrder(input)
				}}
			}
			service = mustService(t, data, geo, planner)
			finish := mustTime("2026-09-17T" + tc.finish + "Z")
			completed, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: "finish-after-late-start", ScenarioID: snapshot.ScenarioID, SnapshotRevision: data.snapshot.Revision, BasePlanID: data.plan.ID,
				Event: contracts.Event{ID: "finish-after-late-start", OccurredAt: finish, Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.replan != (geo.matrixCalls > 0 && len(planner.modes) > 0) {
				t.Fatalf("routing must depend on actual completion vs accepted end, not forecast: geo=%+v modes=%v", geo, planner.modes)
			}
			if !tc.replan && (geo.routesCalls != 0 || geo.positionCalls != 0) {
				t.Fatal("on-time completion requested travel data")
			}
			if !reflect.DeepEqual(completed.Draft.Unassigned, base.Unassigned) {
				t.Fatal("previously unassigned orders changed")
			}
			for _, route := range completed.Draft.Routes {
				if route.EngineerID == "eng-2" && !reflect.DeepEqual(route, base.Routes[1]) {
					t.Fatal("another crew's schedule changed")
				}
				if route.EngineerID == "eng-1" {
					if !route.Visits[0].StartAt.Equal(start) || !route.Visits[0].EndAt.Equal(finish) {
						t.Fatal("actual work interval was not recorded")
					}
					if !tc.replan && (!reflect.DeepEqual(route.Visits[1:], base.Routes[0].Visits[1:]) || !reflect.DeepEqual(route.Legs, base.Routes[0].Legs)) {
						t.Fatal("on-time completion changed future visits or travel")
					}
				}
			}
		})
	}
}

func TestWorkStatusRejectsInvalidTransitions(t *testing.T) {
	for _, tc := range []struct {
		name, at, engineer string
		status             contracts.OrderStatus
	}{
		{"before_arrival", "06:10:00", "eng-1", contracts.OrderStatusInProgress},
		{"before_window", "06:30:00", "eng-1", contracts.OrderStatusInProgress},
		{"wrong_engineer", "07:00:00", "eng-2", contracts.OrderStatusInProgress},
		{"complete_unstarted", "07:20:00", "eng-1", contracts.OrderStatusCompleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := testSnapshot()
			base := testSavedPlan(snapshot)
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{})
			_, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: tc.name, ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
				Event: contracts.Event{ID: tc.name, OccurredAt: mustTime("2026-09-17T" + tc.at + "Z"), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: tc.engineer, Status: tc.status})},
			})
			requireContractCode(t, err, contracts.ErrorEventConflict)
		})
	}
}

func TestWorkStatusLateCompletionStillReplans(t *testing.T) {
	snapshot, base := workStatusFixture()
	departure, start, end := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt, base.Routes[0].Visits[0].EndAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start, ExpectedEndAt: &end}
	// Keep the other crew out of transit so this test focuses on the boundary.
	base.Routes = base.Routes[:1]
	snapshot.Orders = snapshot.Orders[:2]
	base.Unassigned = nil
	geo, planner := standardGeo(), &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	_, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "late-by-one-second", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "late-by-one-second", OccurredAt: end.Add(time.Second), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if geo.matrixCalls == 0 || len(planner.modes) == 0 {
		t.Fatal("late completion skipped necessary replanning")
	}
}

func TestLateCompletionRetainsOtherCrewsElapsedTripOnlyOnce(t *testing.T) {
	snapshot, base := workStatusFixture()
	departure, start := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt
	snapshot.Orders[0].Status = contracts.OrderStatusInProgress
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &start}
	// The other crew's planned arrival is past, but no work start was reported.
	base.AsOf = mustTime("2026-09-17T07:40:00Z")
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "late-with-other-arrival", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "late-with-other-arrival", OccurredAt: mustTime("2026-09-17T07:45:00Z"), Type: contracts.EventOrderStatusChanged, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range result.Draft.Routes {
		if route.EngineerID == "eng-2" {
			if !reflect.DeepEqual(route, base.Routes[1]) {
				t.Fatalf("unrelated crew's route changed: %+v", route)
			}
			return
		}
	}
	t.Fatal("unrelated crew's route disappeared")
}

func TestReplayRemembersTripAcrossStatusOnlySave(t *testing.T) {
	snapshot, base := workStatusFixture()
	// A status on eng-1 advanced the clock while eng-2 was travelling. The
	// next event happens after eng-2's arrival; it must not return to office.
	base.AsOf = mustTime("2026-09-17T07:20:00Z")
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{})
	replay, err := service.replayAt(context.Background(), &snapshot, base, contracts.Event{
		ID: "event-after-arrival", OccurredAt: mustTime("2026-09-17T07:40:00Z"), Type: contracts.EventEngineerUnavailable,
		Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if replay.states["eng-2"].StartLocationID != "loc-1" {
		t.Fatal("another crew's elapsed trip was lost after recording work status")
	}
	order := orderMap(snapshot.Orders)["other-crew-order"]
	if order.Execution == nil || order.Execution.DepartedAt == nil || !order.Execution.DepartedAt.Equal(base.Routes[1].Legs[0].StartAt) {
		t.Fatal("scheduled departure was not preserved")
	}
}
