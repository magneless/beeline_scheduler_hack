package plans

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestBatchUnassignedCancellationIsOneChangeWithoutRouting(t *testing.T) {
	for _, reason := range []string{"client_refusal", "cannot_perform"} {
		t.Run(reason, func(t *testing.T) {
			snapshot, base := workStatusFixture()
			for n, id := range []string{"second", "keep"} {
				order := snapshot.Orders[3]
				order.ID, order.SourceOrder = id, int64(n+5)
				snapshot.Orders = append(snapshot.Orders, order)
				base.Unassigned = append(base.Unassigned, contracts.UnassignedOrder{OrderID: id, ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "No slot"})
			}
			base.DeferredOrderIDs = []string{"unassigned", "second", "keep"}
			base.Metrics = calculateMetrics(base.Routes, base.Unassigned)
			geo, planner := &fakeGeo{}, &fakePlanner{}
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
			event := contracts.Event{ID: "batch", Type: contracts.EventOrderCancelled, OccurredAt: mustTime("2026-09-17T07:05:00Z"), Payload: contracts.EncodePayload(contracts.EventPayload{OrderIDs: []string{"unassigned", "second"}, Reason: reason})}
			result, err := service.Replan(context.Background(), contracts.ReplanRequest{RequestID: "batch", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, Event: event})
			if err != nil {
				t.Fatal(err)
			}
			if geo.matrixCalls != 0 || geo.routesCalls != 0 || geo.positionCalls != 0 || len(planner.modes) != 0 || !reflect.DeepEqual(result.Draft.Routes, base.Routes) {
				t.Fatal("batch cancellation invoked routing or changed routes")
			}
			expected := cloneSnapshot(snapshot)
			expected.Revision++
			expected.Orders[3].Status, expected.Orders[4].Status = contracts.OrderStatusCancelled, contracts.OrderStatusCancelled
			if !reflect.DeepEqual(result.TargetSnapshot, expected) || snapshot.Orders[3].Status != contracts.OrderStatusActive || result.AppliedEvent == nil || string(result.AppliedEvent.Payload) != string(event.Payload) {
				t.Fatal("batch facts, single revision, or history not preserved")
			}
			if !reflect.DeepEqual(result.Draft.Unassigned, base.Unassigned[2:]) || !reflect.DeepEqual(result.Draft.DeferredOrderIDs, []string{"keep"}) || !reflect.DeepEqual(result.Draft.CancelledOrderIDs, []string{"second", "unassigned"}) || len(result.Draft.Changes) != 2 || result.Draft.Metrics.UnassignedCount != 1 {
				t.Fatal("batch selection or remaining unassigned orders incorrect")
			}
		})
	}
}

func TestBatchCancellationRejectsInvalidSelectionAtomically(t *testing.T) {
	for _, tc := range []struct {
		name, payload, code string
	}{
		{"mixed_assigned", `{"order_ids":["unassigned","order-1"],"reason":"client_refusal"}`, contracts.ErrorEventConflict},
		{"missing_order", `{"order_ids":["unassigned","missing"],"reason":"client_refusal"}`, contracts.ErrorEventConflict},
		{"duplicates", `{"order_ids":["unassigned","unassigned"],"reason":"client_refusal"}`, contracts.ErrorInvalidInput},
		{"empty_selection", `{"order_ids":[],"reason":"client_refusal"}`, contracts.ErrorInvalidInput},
		{"ambiguous_selection", `{"order_id":"unassigned","order_ids":["unassigned"],"reason":"client_refusal"}`, contracts.ErrorInvalidInput},
		{"invalid_reason", `{"order_ids":["unassigned"],"reason":"wrong"}`, contracts.ErrorInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, base := workStatusFixture()
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{})
			result, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: tc.name, ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
				Event: contracts.Event{ID: tc.name, Type: contracts.EventOrderCancelled, OccurredAt: base.AsOf, Payload: json.RawMessage(tc.payload)},
			})
			requireContractCode(t, err, tc.code)
			if result.AppliedEvent != nil || result.TargetSnapshot.Revision != 0 || snapshot.Orders[3].Status != contracts.OrderStatusActive {
				t.Fatal("invalid batch partially cancelled orders")
			}
		})
	}
}

func TestUnassignedCancellationPreservesAllRoutesWithoutDependencies(t *testing.T) {
	for _, reason := range []string{"client_refusal", "cannot_perform"} {
		for _, option := range []string{"strict", "original", "reserve"} {
			t.Run(reason+"/"+option, func(t *testing.T) {
				snapshot, base := workStatusFixture()
				// One crew is working without a fresh forecast; another is in
				// transit. Neither execution nor travel should be replayed.
				snapshot.Orders[0].Status = contracts.OrderStatusInProgress
				snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", StartedAt: ptr(base.Routes[0].Visits[0].StartAt)}
				for n, id := range []string{"keep-deferred", "keep-unassigned", "already-cancelled"} {
					order := snapshot.Orders[3]
					order.ID, order.SourceOrder = id, int64(n+5)
					if id == "already-cancelled" {
						order.Status = contracts.OrderStatusCancelled
						base.CancelledOrderIDs = append(base.CancelledOrderIDs, id)
					} else {
						base.Unassigned = append(base.Unassigned, contracts.UnassignedOrder{OrderID: id, ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "Keep this reason"})
					}
					snapshot.Orders = append(snapshot.Orders, order)
				}
				base.DeferredOrderIDs = []string{"unassigned", "keep-deferred"}
				base.SolveMode = contracts.SolveModeOptimized
				base.Metrics = calculateMetrics(base.Routes, base.Unassigned)
				stock, err := equipmentRemaining(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				base.EquipmentRemaining = stock
				geo, planner := &fakeGeo{}, &fakePlanner{}
				service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
				event := contracts.Event{ID: "cancel-unassigned", Type: contracts.EventOrderCancelled, OccurredAt: mustTime("2026-09-17T07:05:00Z"), Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "unassigned", Reason: reason})}
				result, err := service.Replan(context.Background(), contracts.ReplanRequest{
					RequestID: "cancel-unassigned", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID, OptionKey: option, SolveMode: contracts.SolveModeBaseline, Event: event,
				})
				if err != nil {
					t.Fatal(err)
				}
				if geo.matrixCalls != 0 || geo.routesCalls != 0 || geo.positionCalls != 0 || len(planner.modes) != 0 {
					t.Fatal("unassigned cancellation invoked routing")
				}
				if !reflect.DeepEqual(result.Draft.Routes, base.Routes) || !reflect.DeepEqual(result.Draft.EquipmentRemaining, stock) || result.Draft.SolveMode != base.SolveMode {
					t.Fatal("accepted routes, equipment, or their algorithm changed")
				}
				expected := cloneSnapshot(snapshot)
				expected.Revision++
				expected.Orders[3].Status = contracts.OrderStatusCancelled
				if !reflect.DeepEqual(result.TargetSnapshot, expected) || snapshot.Orders[3].Status != contracts.OrderStatusActive {
					t.Fatal("cancellation changed unrelated data or mutated history")
				}
				if !reflect.DeepEqual(result.Draft.Unassigned, base.Unassigned[1:]) || !reflect.DeepEqual(result.Draft.DeferredOrderIDs, []string{"keep-deferred"}) {
					t.Fatal("other unassigned orders, reasons, or categories changed")
				}
				metrics := base.Metrics
				metrics.UnassignedCount--
				if !reflect.DeepEqual(result.Draft.Metrics, metrics) || !reflect.DeepEqual(result.Draft.CancelledOrderIDs, []string{"already-cancelled", "unassigned"}) {
					t.Fatal("incorrect cancellation counters")
				}
				if !reflect.DeepEqual(result.Draft.Changes, []contracts.PlanChange{{OrderID: "unassigned", Reason: contracts.PlanChangeCancelled}}) || result.AppliedEvent == nil || string(result.AppliedEvent.Payload) != string(event.Payload) || result.Draft.BasePlanID == nil || *result.Draft.BasePlanID != base.ID || !result.Draft.AsOf.Equal(event.OccurredAt) {
					t.Fatal("cancellation event, reason, or plan history was lost")
				}
			})
		}
	}
}

func TestUnassignedCancellationWithEmptyRoutes(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	base.Routes = []contracts.Route{}
	base.Unassigned = []contracts.UnassignedOrder{{OrderID: "order-1", ReasonCode: contracts.ReasonNoFeasibleSlot, Message: "No slot"}}
	base.DeferredOrderIDs = []string{"order-1"}
	base.Metrics = calculateMetrics(base.Routes, base.Unassigned)
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{})
	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "empty", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "empty", Type: contracts.EventOrderCancelled, OccurredAt: base.AsOf, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", Reason: "cannot_perform"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Draft.Routes) != 0 || len(result.Draft.Unassigned) != 0 || len(result.Draft.DeferredOrderIDs) != 0 || result.Draft.Metrics.UnassignedCount != 0 {
		t.Fatal("last unassigned order was not removed")
	}
}

func TestUnassignedCancellationRejectsInvalidReason(t *testing.T) {
	snapshot, base := workStatusFixture()
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{})
	_, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "invalid-reason", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
		Event: contracts.Event{ID: "invalid-reason", Type: contracts.EventOrderCancelled, OccurredAt: base.AsOf, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "unassigned", Reason: ""})},
	})
	requireContractCode(t, err, contracts.ErrorInvalidInput)
}
