//go:build ortools

package httpapi

import (
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"testing"
	"time"
)

func TestIntegratedOptimizedBuildReplanHistory(t *testing.T) {
	h, _, f := integrationServerMode(t, c.SolveModeOptimized, true)
	var v c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &v)
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+v.Snapshot.ScenarioID+"/plans", map[string]any{"request_id": "optimized-build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("optimized build failed: %+v", run)
	}
	var plan c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
	if plan.Termination != c.TerminationCompleted && plan.Termination != c.TerminationTimeLimit {
		t.Fatalf("optimized build termination: %s", plan.Termination)
	}
	for _, issue := range plan.Issues {
		if issue.Code == "BASELINE_ONLY" {
			t.Fatal("optimized plan unexpectedly marked baseline-only")
		}
	}
	if len(plan.Routes) == 0 || len(plan.Routes[0].Legs) == 0 || len(plan.Routes[0].Legs[0].Geometry) < 2 {
		t.Fatalf("optimized plan has no geometry: %+v", plan)
	}
	for n, step := range f.Execution.Steps {
		call(t, h, "POST", "/plans/"+plan.ID+"/events", map[string]any{"request_id": step.Request.RequestID, "snapshot_revision": n + 1, "event": step.Request.Event}, 202, &accepted)
		run = awaitRun(t, h, accepted["run_id"])
		if run.Status != "succeeded" {
			t.Fatalf("optimized replan step %d failed: %+v", n, run)
		}
		call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
		if plan.Termination != c.TerminationCompleted && plan.Termination != c.TerminationTimeLimit {
			t.Fatalf("optimized status step termination: %s", plan.Termination)
		}
		for _, issue := range plan.Issues {
			if issue.Code == "BASELINE_ONLY" {
				t.Fatal("optimized status plan unexpectedly marked baseline-only")
			}
		}
	}
	if plan.Metrics.CompletedCount != 1 || plan.SnapshotRevision != 5 || plan.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 0 {
		t.Fatalf("optimized final plan invalid: %+v", plan)
	}
	call(t, h, "GET", "/scenarios/"+v.Snapshot.ScenarioID+"?revision=1", nil, 200, &v)
	if v.Snapshot.Orders[0].Status != c.OrderStatusActive {
		t.Fatal("historical snapshot was modified")
	}
}

func TestIntegratedOptimizedAllEventKinds(t *testing.T) {
	h, _, _ := integrationServerMode(t, c.SolveModeOptimized, true)
	var v c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "east"}, 201, &v)
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+v.Snapshot.ScenarioID+"/plans", map[string]any{"request_id": "optimized-events-build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("optimized build failed: %+v", run)
	}
	var plan c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
	if plan.Termination != c.TerminationCompleted && plan.Termination != c.TerminationTimeLimit {
		t.Fatalf("optimized build termination: %s", plan.Termination)
	}
	order := v.Snapshot.Orders[0]
	order.ID = "optimized-incoming"
	order.WorkType = c.WorkTypeEmergency
	order.Priority = c.PriorityUrgent
	order.RequiredSkills = []string{"emergency"}
	order.ServiceSec = 4800
	order.ReceivedAt = plan.AsOf
	order.Window.Start = plan.AsOf
	order.Window.End = plan.AsOf.Add(23 * time.Hour)
	events := []struct {
		kind    string
		payload any
	}{
		{"urgent_order_added", map[string]any{"order": order}},
		{"order_cancelled", map[string]any{"order_id": "optimized-incoming", "reason": "client_refusal"}},
		{"engineer_unavailable", map[string]any{"engineer_id": v.Snapshot.Engineers[0].ID}},
	}
	for _, event := range events {
		body := map[string]any{"request_id": "optimized-" + event.kind, "snapshot_revision": plan.SnapshotRevision, "event": map[string]any{"id": "optimized-" + event.kind, "occurred_at": plan.AsOf, "type": event.kind, "payload": event.payload}}
		call(t, h, "POST", "/plans/"+plan.ID+"/events", body, 202, &accepted)
		run = awaitRun(t, h, accepted["run_id"])
		if run.Status != "succeeded" {
			t.Fatalf("optimized %s failed: %+v", event.kind, run)
		}
		call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
		if plan.Termination != c.TerminationCompleted && plan.Termination != c.TerminationTimeLimit {
			t.Fatalf("optimized %s termination: %s", event.kind, plan.Termination)
		}
		for _, issue := range plan.Issues {
			if issue.Code == "BASELINE_ONLY" {
				t.Fatalf("optimized %s unexpectedly marked baseline-only", event.kind)
			}
		}
	}
}
