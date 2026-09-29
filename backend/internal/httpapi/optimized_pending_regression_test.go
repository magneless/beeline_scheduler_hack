//go:build vroom

package httpapi

import (
	"context"
	"reflect"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

// Exercise the dispatcher's queued-edit workflow with the production optimized
// constructor, real PostgreSQL persistence, and a binding equipment constraint.
func TestOptimizedPendingChangesPreserveEquipmentAndAcceptedHistory(t *testing.T) {
	h, store, fixture := integrationServerMode(t, c.SolveModeOptimized, true, true)
	snapshot := fixture.Snapshot
	snapshot.ScenarioID = storage.ID("optimized-pending")
	snapshot.Engineers[0].EquipmentStock = map[c.Equipment]int64{c.EquipmentRouter: 2}
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	path := "/scenarios/" + snapshot.ScenarioID
	var proposal storage.Proposal
	call(t, h, "POST", path+"/proposals", map[string]any{
		"request_id": "initial", "snapshot_revision": view.Snapshot.Revision,
		"expected_current_plan_id": nil, "solve_mode": c.SolveModeOptimized,
	}, 201, &proposal)
	var initial c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{
		"request_id": "accept-initial", "option_key": "strict",
	}, 200, &initial)
	if initial.SolveMode != c.SolveModeOptimized || initial.Metrics.AssignedCount != 1 {
		t.Fatalf("initial optimized plan: %+v", initial)
	}
	protected := initial.Routes[0].Visits[0]
	var queue storage.PendingChanges
	for _, id := range []string{"queued-a", "queued-b"} {
		order := snapshot.Orders[0]
		order.ID, order.ReceivedAt = id, initial.AsOf
		queue = queueEvent(t, h, initial, queue.Revision,
			eventBody("add-"+id, "ordinary_order_added", initial.AsOf, map[string]any{"order": order}), 200)
	}
	call(t, h, "GET", path, nil, 200, &view)
	if view.CurrentPlanID == nil || *view.CurrentPlanID != initial.ID || len(view.Snapshot.Orders) != 1 {
		t.Fatal("saving pending edits changed the accepted day")
	}
	var restored storage.PendingChanges
	call(t, h, "GET", path+"/pending-changes", nil, 200, &restored)
	if len(restored.Events) != 2 || restored.Revision != queue.Revision {
		t.Fatal("pending edits were not persisted")
	}
	calculate := func(id string) {
		t.Helper()
		call(t, h, "POST", path+"/proposals", map[string]any{
			"request_id": id, "snapshot_revision": initial.SnapshotRevision,
			"expected_current_plan_id": initial.ID, "pending_revision": queue.Revision,
			"solve_mode": c.SolveModeOptimized,
		}, 201, &proposal)
	}
	calculate("equipment-bound")
	strictFound := false
	for _, option := range proposal.Options {
		if option.Key != "strict" {
			continue
		}
		strictFound = true
		draft := option.Result.Draft
		// The displayed stock is physical stock: reservations constrain future
		// assignments but consumption occurs only when work actually starts.
		if draft.SolveMode != c.SolveModeOptimized || draft.Metrics.AssignedCount != 2 || draft.Metrics.UnassignedCount != 1 || draft.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 2 {
			t.Fatalf("two routers must cover exactly two of three jobs: %+v", draft)
		}
		found := false
		for _, route := range draft.Routes {
			for _, visit := range route.Visits {
				if visit.OrderID == protected.OrderID {
					found = true
					if visit != protected || route.EngineerID != initial.Routes[0].EngineerID {
						t.Fatal("ordinary pending edits changed the accepted appointment")
					}
				}
			}
		}
		if !found {
			t.Fatal("accepted appointment disappeared")
		}
	}
	if !strictFound {
		t.Fatal("strict proposal missing")
	}
	staleProposalID := proposal.ID
	queue = queueEvent(t, h, initial, queue.Revision,
		eventBody("cancel-queued-b", "order_cancelled", initial.AsOf,
			map[string]any{"order_id": "queued-b", "reason": "client_refusal"}), 200)
	call(t, h, "POST", "/proposals/"+staleProposalID+"/accept", map[string]any{
		"request_id": "reject-stale", "option_key": "strict",
	}, 409, nil)
	calculate("after-cancellation")
	accept := map[string]any{"request_id": "accept-pending", "option_key": "strict"}
	var accepted, repeated c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", accept, 200, &accepted)
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", accept, 200, &repeated)
	if !reflect.DeepEqual(accepted, repeated) {
		t.Fatal("repeated acceptance changed the plan or equipment")
	}
	if accepted.SolveMode != c.SolveModeOptimized || accepted.SnapshotRevision != initial.SnapshotRevision+1 || accepted.Metrics.AssignedCount != 2 || accepted.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 2 {
		t.Fatalf("invalid committed optimized plan: %+v", accepted)
	}
	if !reflect.DeepEqual(accepted.CancelledOrderIDs, []string{"queued-b"}) {
		t.Fatal("queued cancellation lost")
	}
	call(t, h, "GET", path+"/pending-changes", nil, 200, &restored)
	if len(restored.Events) != 0 || restored.Revision <= queue.Revision {
		t.Fatal("acceptance did not clear the persisted queue")
	}
	var history c.Plan
	call(t, h, "GET", "/plans/"+initial.ID, nil, 200, &history)
	if !reflect.DeepEqual(history, initial) {
		t.Fatal("acceptance mutated the historical plan")
	}
	call(t, h, "GET", path+"?revision=1", nil, 200, &view)
	if len(view.Snapshot.Orders) != 1 || view.Snapshot.Orders[0].Status != c.OrderStatusActive || view.Snapshot.Engineers[0].EquipmentStock[c.EquipmentRouter] != 2 {
		t.Fatal("historical snapshot or original equipment stock changed")
	}
	for _, status := range []c.OrderStatus{c.OrderStatusInProgress, c.OrderStatusCompleted} {
		at := protected.StartAt
		payload := map[string]any{"order_id": protected.OrderID, "engineer_id": "eng-1", "status": status}
		if status == c.OrderStatusInProgress {
			payload["expected_end_at"] = protected.EndAt
		} else {
			at = protected.EndAt
		}
		body := map[string]any{
			"request_id": "work-" + string(status), "snapshot_revision": accepted.SnapshotRevision,
			"event": eventBody("work-"+string(status), "order_status_changed", at, payload),
		}
		var firstRun, repeatRun map[string]string
		call(t, h, "POST", "/plans/"+accepted.ID+"/events", body, 202, &firstRun)
		run := awaitRun(t, h, firstRun["run_id"])
		if run.Status != "succeeded" {
			t.Fatalf("work status %s failed: %+v", status, run)
		}
		call(t, h, "POST", "/plans/"+accepted.ID+"/events", body, 202, &repeatRun)
		if firstRun["run_id"] != repeatRun["run_id"] {
			t.Fatal("retry created another work-status operation")
		}
		call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &accepted)
		if accepted.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 1 {
			t.Fatalf("work status %s consumed equipment incorrectly: %+v", status, accepted.EquipmentRemaining)
		}
	}
	if accepted.Metrics.CompletedCount != 1 || !reflect.DeepEqual(accepted.CompletedOrderIDs, []string{protected.OrderID}) {
		t.Fatal("confirmed completion was lost")
	}
}
