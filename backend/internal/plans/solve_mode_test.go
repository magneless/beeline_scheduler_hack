package plans

import (
	"context"
	"reflect"
	"testing"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestBuildSelectsAlgorithmPerRequest(t *testing.T) {
	snapshot := testSnapshot()
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot}, standardGeo(), planner)
	for _, mode := range []contracts.SolveMode{contracts.SolveModeBaseline, contracts.SolveModeOptimized} {
		planner.modes = nil
		result, err := service.Build(context.Background(), contracts.BuildPlanRequest{RequestID: "select-mode", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, SolveMode: mode})
		if err != nil {
			t.Fatal(err)
		}
		if result.Draft.SolveMode != mode || !reflect.DeepEqual(planner.modes, []contracts.SolveMode{contracts.SolveModeBaseline, mode}) {
			t.Fatalf("wrong algorithm: saved=%s calls=%v", result.Draft.SolveMode, planner.modes)
		}
	}
	if service.mode != contracts.SolveModeOptimized {
		t.Fatal("request mutated shared service default")
	}
}

func TestRejectAlgorithmBeforeGeo(t *testing.T) {
	snapshot := testSnapshot()
	geo := standardGeo()
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot}, geo, planner)
	_, err := service.Build(context.Background(), contracts.BuildPlanRequest{RequestID: "invalid-mode", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, SolveMode: contracts.SolveModeInsertOnly})
	if err == nil || geo.matrixCalls != 0 || len(planner.modes) != 0 {
		t.Fatalf("invalid mode was not rejected early: %v", err)
	}
}

func TestResolvePlanAlgorithm(t *testing.T) {
	service := &Service{mode: contracts.SolveModeOptimized}
	cases := []struct {
		name      string
		requested contracts.SolveMode
		base      *contracts.Plan
		want      contracts.SolveMode
	}{
		{"default", "", nil, contracts.SolveModeOptimized},
		{"stored", "", &contracts.Plan{PlanDraft: contracts.PlanDraft{SolveMode: contracts.SolveModeBaseline}}, contracts.SolveModeBaseline},
		{"override", contracts.SolveModeOptimized, &contracts.Plan{PlanDraft: contracts.PlanDraft{SolveMode: contracts.SolveModeBaseline}}, contracts.SolveModeOptimized},
		{"legacy", "", &contracts.Plan{PlanDraft: contracts.PlanDraft{Issues: []contracts.Issue{{Code: "BASELINE_ONLY"}}}}, contracts.SolveModeBaseline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.resolveMode(tc.requested, tc.base)
			if err != nil || got != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}
