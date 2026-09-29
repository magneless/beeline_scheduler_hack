package plans

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func workFact(id string, status contracts.OrderStatus, at time.Time) contracts.Event {
	return contracts.Event{ID: fmt.Sprintf("%s-%s", id, status), Type: contracts.EventOrderStatusChanged, OccurredAt: at, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: id, EngineerID: "eng-1", Status: status, ExpectedEndAt: func() *time.Time {
		if status == contracts.OrderStatusInProgress {
			return ptr(at.Add(30 * time.Minute))
		}
		return nil
	}()})}
}

func TestWorkFactsCanBeEnteredInAnyAppointmentOrder(t *testing.T) {
	// Every interleaving that keeps each individual start before its finish.
	for _, sequence := range [][]int{{0, 1, 2, 3}, {0, 2, 1, 3}, {0, 2, 3, 1}, {2, 3, 0, 1}, {2, 0, 3, 1}, {2, 0, 1, 3}} {
		t.Run(fmt.Sprint(sequence), func(t *testing.T) {
			snapshot, base := workStatusFixture()
			original := clonePlanRoutes(base.Routes)
			first, next := base.Routes[0].Visits[0], base.Routes[0].Visits[1]
			events := []contracts.Event{workFact(first.OrderID, contracts.OrderStatusInProgress, first.StartAt), workFact(first.OrderID, contracts.OrderStatusCompleted, first.EndAt), workFact(next.OrderID, contracts.OrderStatusInProgress, next.StartAt), workFact(next.OrderID, contracts.OrderStatusCompleted, next.EndAt)}
			data, geo, planner := &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{}
			service := mustService(t, data, geo, planner)
			clock := base.AsOf
			for index, i := range sequence {
				event := events[i]
				clock = laterTime(clock, event.OccurredAt)
				result, err := service.Replan(context.Background(), contracts.ReplanRequest{RequestID: event.ID, ScenarioID: snapshot.ScenarioID, SnapshotRevision: data.snapshot.Revision, BasePlanID: data.plan.ID, Event: event})
				if err != nil {
					t.Fatal(err)
				}
				if !result.Draft.AsOf.Equal(clock) || !result.AppliedEvent.OccurredAt.Equal(event.OccurredAt) {
					t.Fatal("fact time or plan clock changed")
				}
				if !reflect.DeepEqual(result.Draft.Routes, original) || !reflect.DeepEqual(result.Draft.Unassigned, base.Unassigned) {
					t.Fatal("recording facts changed accepted appointments")
				}
				data.snapshot, data.plan = result.TargetSnapshot, contracts.Plan{ID: fmt.Sprint(index), PlanDraft: result.Draft}
			}
			if data.plan.Metrics.CompletedCount != 2 || data.plan.EquipmentRemaining["eng-1"][contracts.EquipmentRouter] != 1 {
				t.Fatal("completion count or inventory depends on entry order")
			}
			if geo.matrixCalls+geo.routesCalls+geo.positionCalls != 0 || len(planner.modes) != 0 {
				t.Fatal("recording on-time facts invoked routing")
			}
		})
	}
}

func TestHistoricalWorkFactsKeepChronologyChecks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		order  string
		status contracts.OrderStatus
		at     string
		valid  bool
	}{
		{"earlier start", "order-1", contracts.OrderStatusInProgress, "07:00:00", true},
		{"overlapping start", "order-1", contracts.OrderStatusInProgress, "08:10:00", false},
		{"same start", "order-1", contracts.OrderStatusInProgress, "08:00:00", false},
		{"before arrival", "order-1", contracts.OrderStatusInProgress, "06:00:00", false},
		{"overlapping finish", "order-1", contracts.OrderStatusCompleted, "08:10:00", false},
		{"touching boundary", "order-1", contracts.OrderStatusCompleted, "08:00:00", true},
		{"zero duration", "order-1", contracts.OrderStatusCompleted, "07:00:00", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, base := workStatusFixture()
			next := &snapshot.Orders[1]
			next.Status, next.Execution = contracts.OrderStatusCompleted, &contracts.OrderExecution{EngineerID: "eng-1", StartedAt: ptr(base.Routes[0].Visits[1].StartAt), FinishedAt: ptr(base.Routes[0].Visits[1].EndAt)}
			base.AsOf = *next.Execution.FinishedAt
			if tc.status == contracts.OrderStatusCompleted {
				snapshot.Orders[0].Status = contracts.OrderStatusInProgress
				snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", StartedAt: ptr(base.Routes[0].Visits[0].StartAt), DepartedAt: ptr(base.Routes[0].Legs[0].StartAt)}
			}
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
			result, err := service.Replan(context.Background(), contracts.ReplanRequest{RequestID: tc.name, ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, Event: workFact(tc.order, tc.status, mustTime("2026-09-17T"+tc.at+"Z"))})
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				if !result.Draft.AsOf.Equal(base.AsOf) {
					t.Fatal("plan clock moved backwards")
				}
			} else if err == nil {
				t.Fatal("invalid fact accepted")
			}
		})
	}
}

func TestQueuedHistoricalFactsPreserveEntryOrderForUndo(t *testing.T) {
	snapshot, base := workStatusFixture()
	base.AsOf = mustTime("2026-09-17T10:00:00Z")
	events := []contracts.Event{workFact("next", contracts.OrderStatusInProgress, mustTime("2026-09-17T08:00:00Z")), workFact("next", contracts.OrderStatusCompleted, mustTime("2026-09-17T08:30:00Z")), workFact("order-1", contracts.OrderStatusInProgress, mustTime("2026-09-17T07:00:00Z")), workFact("order-1", contracts.OrderStatusCompleted, mustTime("2026-09-17T07:30:00Z"))}
	batch := queued(events...)
	batch.OccurredAt = events[1].OccurredAt
	geo, planner := &fakeGeo{}, &fakePlanner{}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	input := contracts.ReplanRequest{RequestID: "historical-batch", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, Event: batch}
	preview, normalized, err := service.PreparePending(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Orders[0].Status != contracts.OrderStatusCompleted || preview.Orders[1].Status != contracts.OrderStatusCompleted || !reflect.DeepEqual(eventList(normalized), events) {
		t.Fatal("facts or entry order lost")
	}
	result, err := service.Replan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Draft.AsOf.Equal(base.AsOf) || !reflect.DeepEqual(result.Draft.Routes, base.Routes) {
		t.Fatal("historical queue changed clock or appointments")
	}
	if geo.matrixCalls+geo.routesCalls+geo.positionCalls != 0 || len(planner.modes) != 0 {
		t.Fatal("fact queue invoked routing")
	}
}

func TestMissingHistoricalFinishDoesNotBecomeAnInventedFact(t *testing.T) {
	snapshot, base := workStatusFixture()
	for i, visit := range base.Routes[0].Visits {
		snapshot.Orders[i].Status = contracts.OrderStatusInProgress
		snapshot.Orders[i].Execution = &contracts.OrderExecution{EngineerID: "eng-1", StartedAt: ptr(visit.StartAt), ExpectedEndAt: ptr(visit.EndAt)}
	}
	base.AsOf = base.Routes[0].Visits[1].StartAt
	newOrder := snapshot.Orders[0]
	newOrder.ID, newOrder.Status, newOrder.Execution = "new", contracts.OrderStatusActive, nil
	newOrder.WorkType = contracts.WorkTypeRepair
	newOrder.ReceivedAt = base.AsOf
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{})
	_, err := service.Replan(context.Background(), contracts.ReplanRequest{RequestID: "new-with-missing-finish", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "new", Type: contracts.EventOrdinaryOrderAdded, OccurredAt: base.AsOf, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &newOrder})}})
	var conflict *contracts.ContractError
	if !errors.As(err, &conflict) || conflict.Code != contracts.ErrorEventConflict || conflict.Details["order_id"] != "order-1" {
		t.Fatalf("expected missing historical finish, got %v", err)
	}
	if snapshot.Orders[0].Execution.FinishedAt != nil {
		t.Fatal("missing completion fabricated")
	}
}
