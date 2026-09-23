package storage

import (
	"context"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"testing"
)

func TestReplaceEngineersStaleAndSuccessful(t *testing.T) {
	s := testStore(t)
	v, _ := seed(t, s)
	roster := []c.Engineer{{ID: "replacement", Skills: []string{}, Transport: c.TransportWalk, Available: true, Shift: c.Window{Start: v.Snapshot.Orders[0].Window.Start, End: v.Snapshot.Orders[0].Window.End}, EquipmentStock: map[c.Equipment]int64{}}}
	got, err := s.ReplaceEngineers(context.Background(), v.Snapshot.ScenarioID, v.Snapshot.Revision, roster)
	if err != nil {
		t.Fatal(err)
	}
	if got.Snapshot.Revision != v.Snapshot.Revision+1 || got.CurrentPlanID != nil || len(got.Snapshot.Engineers) != 1 {
		t.Fatalf("unexpected replacement: %+v", got)
	}
	if _, err = s.ReplaceEngineers(context.Background(), v.Snapshot.ScenarioID, v.Snapshot.Revision, roster); err == nil {
		t.Fatal("stale replacement accepted")
	} else {
		requireCode(t, err, "STALE_VERSION")
	}
	if _, err = s.ReplaceEngineers(context.Background(), v.Snapshot.ScenarioID, got.Snapshot.Revision, nil); err == nil {
		t.Fatal("empty replacement accepted")
	} else {
		requireCode(t, err, "INVALID_INPUT")
	}
}
