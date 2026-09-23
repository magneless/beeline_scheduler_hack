package contracts

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBackendExamplesMatchContractTypes(t *testing.T) {
	example := loadExample(t, "backend_flow.json")
	tests := map[string]any{
		"snapshot":         &Snapshot{},
		"build_request":    &BuildPlanRequest{},
		"geocode_request":  &GeocodeRequest{},
		"geocode_result":   &GeocodeResult{},
		"matrix_request":   &MatrixRequest{},
		"matrix":           &TravelMatrix{},
		"solve_request":    &SolveRequest{},
		"solve_result":     &SolveResult{},
		"routes_request":   &RoutesRequest{},
		"routes_geometry":  &RoutesGeometry{},
		"position_request": &PositionRequest{},
		"position_result":  &PositionResult{},
		"plan_result":      &PlanResult{},
		"plan_commit":      &PlanCommit{},
		"saved_plan":       &Plan{},
		"replan_request":   &ReplanRequest{},
		"replan_result":    &PlanResult{},
	}
	for key, target := range tests {
		t.Run(key, func(t *testing.T) {
			unmarshalExample(t, example, key, target)
		})
	}
}

func TestFrontendExamplesMatchSharedTypes(t *testing.T) {
	example := loadExample(t, "frontend_flow.json")
	tests := map[string]any{
		"scenario":        &ScenarioView{},
		"run":             &Run{},
		"plan":            &Plan{},
		"scenario_before": &ScenarioView{},
		"run_running":     &Run{},
		"event_request": &struct {
			RequestID        string `json:"request_id"`
			SnapshotRevision int64  `json:"snapshot_revision"`
			Event            Event  `json:"event"`
		}{},
		"event_plan":           &Plan{},
		"event_run":            &Run{},
		"scenario_after_event": &ScenarioView{},
		"import_with_issue":    &ScenarioView{},
		"stale_version_error":  &ContractError{},
	}
	for key, target := range tests {
		t.Run(key, func(t *testing.T) {
			unmarshalExample(t, example, key, target)
		})
	}
}

func loadExample(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	path := filepath.Join(filepath.Dir(currentFile), "..", "docs", "contracts", "examples", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return result
}

func unmarshalExample(t *testing.T, example map[string]json.RawMessage, key string, target any) {
	t.Helper()
	raw, exists := example[key]
	if !exists {
		t.Fatalf("example key %q is missing", key)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode %q: %v", key, err)
	}
}
