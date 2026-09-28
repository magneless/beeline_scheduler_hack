package storage

import (
	"context"
	"sync"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

func TestProposalPreviewAndAtomicAcceptance(t *testing.T) {
	s := testStore(t)
	view, fixture := seed(t, s)
	ctx := context.Background()
	service := &testkit.Plans{Reader: s, Fixture: fixture}
	result, e := service.Build(ctx, c.BuildPlanRequest{ScenarioID: view.Snapshot.ScenarioID, SnapshotRevision: 1})
	if e != nil {
		t.Fatal(e)
	}
	input := ProposalInput{ScenarioID: view.Snapshot.ScenarioID, RequestID: "preview", SnapshotRevision: 1}
	p, e := s.SaveProposal(ctx, input, []c.PlanOption{{Key: "strict", Label: "В окнах", Result: result}})
	if e != nil {
		t.Fatal(e)
	}
	current, e := s.CurrentProposal(ctx, input.ScenarioID)
	if e != nil || current == nil || current.ID != p.ID {
		t.Fatalf("pending proposal: %+v, %v", current, e)
	}
	view, e = s.GetScenario(ctx, input.ScenarioID, 0)
	if e != nil || view.CurrentPlanID != nil || view.Snapshot.Revision != 1 {
		t.Fatalf("preview changed scenario: %+v, %v", view, e)
	}
	_, e = s.AcceptProposal(ctx, p.ID, "accept-invalid", "unknown")
	requireCode(t, e, "INVALID_INPUT")
	plan, e := s.AcceptProposal(ctx, p.ID, "accept", "strict")
	if e != nil {
		t.Fatal(e)
	}
	if !same(plan.PlanDraft, result.Draft) {
		t.Fatal("accepted plan differs from stored selected result")
	}
	again, e := s.AcceptProposal(ctx, p.ID, "accept", "strict")
	if e != nil || again.ID != plan.ID {
		t.Fatalf("idempotent accept: %+v, %v", again, e)
	}
	_, e = s.AcceptProposal(ctx, p.ID, "other", "strict")
	requireCode(t, e, "STALE_VERSION")
	current, e = s.CurrentProposal(ctx, input.ScenarioID)
	if e != nil || current != nil {
		t.Fatalf("accepted proposal is still pending: %+v, %v", current, e)
	}
	view, e = s.GetScenario(ctx, input.ScenarioID, 0)
	if e != nil || view.CurrentPlanID == nil || *view.CurrentPlanID != plan.ID {
		t.Fatalf("plan did not become current: %+v, %v", view, e)
	}
}

func TestCompetingProposalAcceptsCommitOnlyOneVariant(t *testing.T) {
	s := testStore(t)
	view, fixture := seed(t, s)
	ctx := context.Background()
	service := &testkit.Plans{Reader: s, Fixture: fixture}
	strict, e := service.Build(ctx, c.BuildPlanRequest{ScenarioID: view.Snapshot.ScenarioID, SnapshotRevision: 1})
	if e != nil {
		t.Fatal(e)
	}
	strict.Draft.OptionKey = "strict"
	reserve := strict
	reserve.Draft.OptionKey = "reserve"
	p, e := s.SaveProposal(ctx, ProposalInput{ScenarioID: view.Snapshot.ScenarioID, RequestID: "preview", SnapshotRevision: 1}, []c.PlanOption{
		{Key: "strict", Result: strict}, {Key: "reserve", Result: reserve},
	})
	if e != nil {
		t.Fatal(e)
	}
	type result struct {
		plan c.Plan
		err  error
	}
	ch := make(chan result, 2)
	var wg sync.WaitGroup
	for _, key := range []string{"strict", "reserve"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			plan, err := s.AcceptProposal(ctx, p.ID, "accept-"+key, key)
			ch <- result{plan, err}
		}(key)
	}
	wg.Wait()
	close(ch)
	success, stale := 0, 0
	for got := range ch {
		if got.err == nil {
			success++
			if got.plan.OptionKey != "strict" && got.plan.OptionKey != "reserve" {
				t.Fatalf("unknown accepted option: %+v", got.plan)
			}
		} else {
			requireCode(t, got.err, "STALE_VERSION")
			stale++
		}
	}
	if success != 1 || stale != 1 {
		t.Fatalf("success=%d stale=%d", success, stale)
	}
	var count int
	if e = s.db.QueryRowContext(ctx, "SELECT count(*) FROM plans WHERE scenario_id=$1", view.Snapshot.ScenarioID).Scan(&count); e != nil || count != 1 {
		t.Fatalf("committed plans=%d, %v", count, e)
	}
}

func TestProposalAcceptanceRejectsStaleScenario(t *testing.T) {
	s := testStore(t)
	view, fixture := seed(t, s)
	ctx := context.Background()
	service := &testkit.Plans{Reader: s, Fixture: fixture}
	result, e := service.Build(ctx, c.BuildPlanRequest{ScenarioID: view.Snapshot.ScenarioID, SnapshotRevision: 1})
	if e != nil {
		t.Fatal(e)
	}
	input := ProposalInput{ScenarioID: view.Snapshot.ScenarioID, RequestID: "preview", SnapshotRevision: 1}
	p, e := s.SaveProposal(ctx, input, []c.PlanOption{{Key: "strict", Result: result}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.PatchEngineer(ctx, input.ScenarioID, view.Snapshot.Engineers[0].ID, 1, func(eng *c.Engineer) error { eng.Available = false; return nil })
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.AcceptProposal(ctx, p.ID, "accept", "strict")
	requireCode(t, e, "STALE_VERSION")
	current, e := s.CurrentProposal(ctx, input.ScenarioID)
	if e != nil || current != nil {
		t.Fatalf("stale proposal shown as current: %+v, %v", current, e)
	}
}
