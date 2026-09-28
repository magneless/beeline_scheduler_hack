package httpapi

import (
	"context"
	"reflect"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

func TestHTTPBatchCancellationAtomicAndIdempotent(t *testing.T) {
	h, store, fixture := integrationServer(t, true, true)
	snapshot := fixture.Snapshot
	snapshot.ScenarioID = storage.ID("scenario")
	for n, id := range []string{"batch-a", "batch-b", "keep"} {
		order := snapshot.Orders[0]
		order.ID, order.SourceOrder = id, int64(n+2)
		order.RequiredSkills = []string{"no-crew-has-this-skill"}
		snapshot.Orders = append(snapshot.Orders, order)
	}
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	sid := view.Snapshot.ScenarioID
	var proposal storage.Proposal
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{"request_id": "build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 201, &proposal)
	var initial c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept", "option_key": "strict"}, 200, &initial)
	path := "/plans/" + initial.ID + "/events"
	for _, tc := range []struct {
		name   string
		ids    []string
		status int
	}{
		{"assigned", []string{"batch-a", "order-1"}, 409},
		{"unknown", []string{"batch-a", "missing"}, 409},
		{"duplicate", []string{"batch-a", "batch-a"}, 422},
		{"empty", []string{}, 422},
	} {
		call(t, h, "POST", path, map[string]any{
			"request_id": tc.name, "snapshot_revision": 1,
			"event": map[string]any{"id": tc.name, "occurred_at": initial.AsOf, "type": "order_cancelled", "payload": map[string]any{"order_ids": tc.ids, "reason": "client_refusal"}},
		}, tc.status, nil)
		call(t, h, "GET", "/scenarios/"+sid, nil, 200, &view)
		if view.Snapshot.Revision != 1 || view.Snapshot.Orders[1].Status != c.OrderStatusActive {
			t.Fatal("invalid selection partially committed")
		}
	}
	payload := map[string]any{"order_ids": []string{"batch-b", "batch-a"}, "reason": "cannot_perform"}
	body := map[string]any{"request_id": "batch", "snapshot_revision": 1, "event": map[string]any{"id": "batch", "occurred_at": initial.AsOf, "type": "order_cancelled", "payload": payload}}
	var accepted map[string]string
	call(t, h, "POST", path, body, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("batch failed: %+v", run.Error)
	}
	// The same selection in another order is the same idempotent command.
	payload["order_ids"] = []string{"batch-a", "batch-b"}
	var repeated map[string]string
	call(t, h, "POST", path, body, 202, &repeated)
	if repeated["run_id"] != accepted["run_id"] {
		t.Fatal("batch retry created a second run")
	}
	var result c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &result)
	call(t, h, "GET", "/scenarios/"+sid, nil, 200, &view)
	if view.Snapshot.Revision != 2 || result.SnapshotRevision != 2 || view.Snapshot.Orders[1].Status != c.OrderStatusCancelled || view.Snapshot.Orders[2].Status != c.OrderStatusCancelled || view.Snapshot.Orders[3].Status != c.OrderStatusActive || !reflect.DeepEqual(result.Routes, initial.Routes) || result.Metrics.UnassignedCount != 1 {
		t.Fatal("batch was not a single atomic change with preserved routes")
	}
	call(t, h, "GET", "/scenarios/"+sid+"?revision=1", nil, 200, &view)
	if view.Snapshot.Orders[1].Status != c.OrderStatusActive || view.Snapshot.Orders[2].Status != c.OrderStatusActive {
		t.Fatal("batch mutated historical data")
	}
}

func TestUnassignedCancellationCommitsDirectlyWithProposalAPIEnabled(t *testing.T) {
	h, store, fixture := integrationServer(t, true, true)
	snapshot := fixture.Snapshot
	snapshot.ScenarioID = storage.ID("scenario")
	for n, id := range []string{"unassigned-refusal", "unassigned-impossible"} {
		order := snapshot.Orders[0]
		order.ID, order.SourceOrder = id, int64(n+2)
		order.RequiredSkills = []string{"no-crew-has-this-skill"}
		snapshot.Orders = append(snapshot.Orders, order)
	}
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	sid := view.Snapshot.ScenarioID
	var proposal storage.Proposal
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{"request_id": "build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 201, &proposal)
	var plan c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept", "option_key": "strict"}, 200, &plan)
	if len(plan.Routes) == 0 || len(plan.Unassigned) != 2 {
		t.Fatalf("fixture must include scheduled and unassigned work: %+v", plan.Metrics)
	}
	initial := plan
	for n, reason := range []string{"client_refusal", "cannot_perform"} {
		orderID := []string{"unassigned-refusal", "unassigned-impossible"}[n]
		body := map[string]any{
			"request_id": reason, "snapshot_revision": plan.SnapshotRevision,
			"event": map[string]any{"id": reason, "occurred_at": initial.AsOf.Add(9 * time.Hour), "type": "order_cancelled", "payload": map[string]any{"order_id": orderID, "reason": reason}},
		}
		path := "/plans/" + plan.ID + "/events"
		var accepted map[string]string
		call(t, h, "POST", path, body, 202, &accepted)
		run := awaitRun(t, h, accepted["run_id"])
		if run.Status != "succeeded" {
			t.Fatalf("unassigned cancellation failed: %+v", run.Error)
		}
		var repeated map[string]string
		call(t, h, "POST", path, body, 202, &repeated)
		if repeated["run_id"] != accepted["run_id"] {
			t.Fatal("repeat cancellation created another run")
		}
		call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
		if !reflect.DeepEqual(plan.Routes, initial.Routes) || !reflect.DeepEqual(plan.EquipmentRemaining, initial.EquipmentRemaining) || plan.Metrics.TotalDistanceM != initial.Metrics.TotalDistanceM || plan.Metrics.AssignedCount != initial.Metrics.AssignedCount || plan.Metrics.UnassignedCount != 1-n {
			t.Fatal("cancelling unassigned work changed crew routes or incorrect counters")
		}
		call(t, h, "GET", "/scenarios/"+sid, nil, 200, &view)
		if view.Snapshot.Revision != int64(n+2) || view.CurrentPlanID == nil || *view.CurrentPlanID != plan.ID || view.Snapshot.Orders[n+1].Status != c.OrderStatusCancelled {
			t.Fatal("cancellation did not update the accepted snapshot and plan")
		}
	}
	for _, id := range []string{"order-1", "unknown", "unassigned-refusal"} {
		call(t, h, "POST", "/plans/"+plan.ID+"/events", map[string]any{
			"request_id": "reject-" + id, "snapshot_revision": plan.SnapshotRevision,
			"event": map[string]any{"id": "reject-" + id, "occurred_at": plan.AsOf, "type": "order_cancelled", "payload": map[string]any{"order_id": id, "reason": "client_refusal"}},
		}, 409, nil)
	}
	call(t, h, "GET", "/scenarios/"+sid+"?revision=1", nil, 200, &view)
	for _, order := range view.Snapshot.Orders {
		if order.Status != c.OrderStatusActive {
			t.Fatal("historical snapshot was modified")
		}
	}
}
