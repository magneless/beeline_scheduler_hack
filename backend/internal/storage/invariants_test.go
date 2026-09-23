package storage

import (
	"context"
	"encoding/json"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestCommandComparisonPreservesLargeIntegers(t *testing.T) {
	a := json.RawMessage(`{"revision":9007199254740992,"a":1}`)
	b := json.RawMessage(`{"a":1,"revision":9007199254740993}`)
	if same(a, b) {
		t.Fatal("different int64 values compared equal")
	}
	if !same(a, json.RawMessage(`{"a":1,"revision":9007199254740992}`)) {
		t.Fatal("key ordering changed identity")
	}
}
func TestWorkerOwnership(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	release, e := s.AcquireWorker(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	_, e = s.AcquireWorker(ctx)
	requireCode(t, e, "EVENT_CONFLICT")
}
func TestMidnightEventBlocksBuildAndPatch(t *testing.T) {
	s := testStore(t)
	v, f := seed(t, s)
	ctx := context.Background()
	in, _ := buildCommit(t, s, v, f, "build")
	p, e := s.CommitPlan(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	event := c.Event{ID: "midnight", OccurredAt: p.AsOf, Type: "engineer_unavailable", Payload: json.RawMessage(`{"engineer_id":"eng-1"}`)}
	req := c.ReplanRequest{RequestID: "midnight", ScenarioID: p.ScenarioID, SnapshotRevision: 1, BasePlanID: p.ID, Event: event}
	if _, e = s.Register(ctx, Command{Kind: "replan", Replan: &req}); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Claim(ctx); e != nil {
		t.Fatal(e)
	}
	result := in.Result
	result.TargetSnapshot.Revision = 2
	result.TargetSnapshot.Engineers[0].Available = false
	result.Draft.SnapshotRevision = 2
	result.Draft.BasePlanID = &p.ID
	result.AppliedEvent = &event
	if _, e = s.CommitPlan(ctx, c.PlanCommit{RequestID: req.RequestID, ExpectedRevision: 1, ExpectedCurrentPlanID: &p.ID, Result: result}); e != nil {
		t.Fatal(e)
	}
	now, _ := s.GetScenario(ctx, p.ScenarioID, 0)
	_, e = s.Register(ctx, command(now, "blocked"))
	requireCode(t, e, "EVENT_CONFLICT")
	_, e = s.PatchEngineer(ctx, p.ScenarioID, "eng-1", 2, func(*c.Engineer) error { return nil })
	requireCode(t, e, "EVENT_CONFLICT")
}
