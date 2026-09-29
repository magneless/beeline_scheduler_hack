//go:build vroom

package httpapi

import (
	"fmt"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestAlgorithmSelectionSurvivesBuildAndEvents(t *testing.T) {
	h, _, fixture := integrationServerMode(t, c.SolveModeOptimized, true)
	var view c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &view)
	path := "/scenarios/" + view.Snapshot.ScenarioID + "/plans"
	for _, invalidMode := range []string{"insert_only", "unknown"} {
		call(t, h, "POST", path, map[string]any{"request_id": "invalid", "snapshot_revision": 1, "expected_current_plan_id": nil, "solve_mode": invalidMode}, 422, nil)
	}
	var accepted map[string]string
	body := map[string]any{"request_id": "algorithm-build", "snapshot_revision": 1, "expected_current_plan_id": nil, "solve_mode": "baseline"}
	call(t, h, "POST", path, body, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("build failed: %+v", run)
	}
	var plan c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
	if plan.SolveMode != c.SolveModeBaseline {
		t.Fatalf("build ignored algorithm: %s", plan.SolveMode)
	}
	body["solve_mode"] = "optimized"
	call(t, h, "POST", path, body, 409, nil)
	// A request without a mode inherits baseline, then an explicit change is
	// persisted and inherited by subsequent status events.
	for n, stepIndex := range []int{0, 2, 3} {
		step := fixture.Execution.Steps[stepIndex]
		eventBody := map[string]any{"request_id": fmt.Sprintf("algorithm-event-%d", n), "snapshot_revision": plan.SnapshotRevision, "event": step.Request.Event}
		want := c.SolveModeBaseline
		if n == 1 {
			eventBody["solve_mode"] = "optimized"
		}
		if n >= 1 {
			want = c.SolveModeOptimized
		}
		if n == 0 {
			eventBody["solve_mode"] = "insert_only"
			call(t, h, "POST", "/plans/"+plan.ID+"/events", eventBody, 422, nil)
			delete(eventBody, "solve_mode")
		}
		call(t, h, "POST", "/plans/"+plan.ID+"/events", eventBody, 202, &accepted)
		run = awaitRun(t, h, accepted["run_id"])
		if run.Status != "succeeded" {
			t.Fatalf("event %d failed: %+v", n, run)
		}
		call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
		if plan.SolveMode != want {
			t.Fatalf("event %d algorithm=%s want=%s", n, plan.SolveMode, want)
		}
	}
	var comparison struct {
		Snapshot  c.Snapshot `json:"snapshot"`
		Baseline  c.Plan     `json:"baseline"`
		Optimized c.Plan     `json:"optimized"`
	}
	call(t, h, "POST", "/scenarios/"+view.Snapshot.ScenarioID+"/plans/compare", map[string]any{"expected_current_plan_id": plan.ID}, 200, &comparison)
	if comparison.Snapshot.Revision != 1 || comparison.Baseline.SolveMode != c.SolveModeBaseline || comparison.Optimized.SolveMode != c.SolveModeOptimized {
		t.Fatalf("comparison did not use the original snapshot and both algorithms: %+v", comparison)
	}
	if len(comparison.Baseline.Routes) == 0 || len(comparison.Optimized.Routes) == 0 || len(comparison.Optimized.Routes[0].Legs[0].Geometry) == 0 {
		t.Fatalf("comparison routes are incomplete: %+v", comparison)
	}
	var after c.ScenarioView
	call(t, h, "GET", "/scenarios/"+view.Snapshot.ScenarioID, nil, 200, &after)
	if after.CurrentPlanID == nil || *after.CurrentPlanID != plan.ID || after.Snapshot.Revision != plan.SnapshotRevision {
		t.Fatal("comparison changed the working plan")
	}
	call(t, h, "POST", "/scenarios/"+view.Snapshot.ScenarioID+"/plans/compare", map[string]any{"expected_current_plan_id": "stale"}, 409, nil)
}
