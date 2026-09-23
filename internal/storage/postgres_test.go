package storage

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/internal/data"
	"github.com/magneless/beeline_scheduler_hack/internal/testkit"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, e := Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "test_" + ID("s")[2:]
	if _, e = admin.db.ExecContext(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close(); admin.db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	return s
}
func seed(t *testing.T, s *Store) (c.ScenarioView, testkit.Fixture) {
	t.Helper()
	f, e := testkit.Load("../../docs/contracts/examples/backend_flow.json")
	if e != nil {
		t.Fatal(e)
	}
	snap := data.Clone(f.Snapshot)
	snap.ScenarioID = ID("scenario")
	v, e := s.CreateScenario(context.Background(), snap, map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	return v, f
}
func command(v c.ScenarioView, rid string) Command {
	return Command{Kind: "build", Build: &c.BuildPlanRequest{RequestID: rid, ScenarioID: v.Snapshot.ScenarioID, SnapshotRevision: v.Snapshot.Revision, ExpectedCurrentPlanID: v.CurrentPlanID}}
}
func buildCommit(t *testing.T, s *Store, v c.ScenarioView, f testkit.Fixture, rid string) (c.PlanCommit, string) {
	t.Helper()
	ctx := context.Background()
	id, e := s.Register(ctx, command(v, rid))
	if e != nil {
		t.Fatal(e)
	}
	claimed, cmd, e := s.Claim(ctx)
	if e != nil || claimed != id {
		t.Fatalf("claim %s %v", claimed, e)
	}
	p := testkit.Plans{Reader: s, Fixture: f}
	result, e := p.Build(ctx, *cmd.Build)
	if e != nil {
		t.Fatal(e)
	}
	return c.PlanCommit{RequestID: rid, ExpectedRevision: v.Snapshot.Revision, ExpectedCurrentPlanID: v.CurrentPlanID, Result: result}, id
}
func requireCode(t *testing.T, e error, code string) {
	t.Helper()
	var ce *c.ContractError
	if !errors.As(e, &ce) || ce.Code != code {
		t.Fatalf("wanted %s, got %v", code, e)
	}
}
func TestConcurrentIdempotencyAndVersions(t *testing.T) {
	s := testStore(t)
	v, f := seed(t, s)
	ctx := context.Background()
	cmd := command(v, "same")
	ids := make(chan string, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for n := 0; n < 12; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); id, e := s.Register(ctx, cmd); ids <- id; errs <- e }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	id := ""
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("multiple runs")
		}
		id = got
	}
	claimed, _, e := s.Claim(ctx)
	if e != nil || claimed != id {
		t.Fatal(e)
	}
	plans := testkit.Plans{Reader: s, Fixture: f}
	result, e := plans.Build(ctx, *cmd.Build)
	if e != nil {
		t.Fatal(e)
	}
	in := c.PlanCommit{RequestID: "same", ExpectedRevision: 1, Result: result}
	p, e := s.CommitPlan(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.CommitPlan(ctx, in)
	if e != nil || again.ID != p.ID {
		t.Fatalf("repeat commit %v", e)
	}
	sameID, e := s.Register(ctx, cmd)
	if e != nil || sameID != id {
		t.Fatalf("repeat after version change %v", e)
	}
	bad := data.Clone(cmd)
	bad.Build.SnapshotRevision = 2
	_, e = s.Register(ctx, bad)
	requireCode(t, e, "IDEMPOTENCY_CONFLICT")
	v2, e := s.GetScenario(ctx, v.Snapshot.ScenarioID, 0)
	if e != nil || v2.Snapshot.Revision != 1 {
		t.Fatalf("build changed revision %v", e)
	}
	r, e := s.GetRun(ctx, id)
	if e != nil || r.Status != "succeeded" || r.PlanID == nil {
		t.Fatalf("run %+v %v", r, e)
	}
	_, e = s.GetSnapshot(ctx, v.Snapshot.ScenarioID, 2)
	requireCode(t, e, "NOT_FOUND")
}
func TestEventAtomicRollbackAndHistory(t *testing.T) {
	s := testStore(t)
	v, f := seed(t, s)
	ctx := context.Background()
	in, _ := buildCommit(t, s, v, f, "build")
	p, e := s.CommitPlan(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	req := data.Clone(f.ReplanRequest)
	req.ScenarioID = v.Snapshot.ScenarioID
	req.BasePlanID = p.ID
	req.RequestID = "cancel"
	req.SnapshotRevision = 1
	cmd := Command{Kind: "replan", Replan: &req}
	rid, e := s.Register(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Claim(ctx); e != nil {
		t.Fatal(e)
	}
	svc := testkit.Plans{Reader: s, Fixture: f}
	result, e := svc.Replan(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	commit := c.PlanCommit{RequestID: req.RequestID, ExpectedRevision: 1, ExpectedCurrentPlanID: &p.ID, Result: result}
	// Force a failure after snapshot/plan/event writes, before transaction commit.
	_, e = s.db.ExecContext(ctx, "CREATE FUNCTION fail_success() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN IF NEW.status = ''succeeded'' THEN RAISE EXCEPTION ''injected failure''; END IF; RETURN NEW; END'; CREATE TRIGGER fail_success BEFORE UPDATE ON runs FOR EACH ROW EXECUTE FUNCTION fail_success()")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.CommitPlan(ctx, commit); e == nil {
		t.Fatal("injected failure ignored")
	}
	current, e := s.GetScenario(ctx, v.Snapshot.ScenarioID, 0)
	if e != nil || current.Snapshot.Revision != 1 || *current.CurrentPlanID != p.ID {
		t.Fatal("partial commit")
	}
	_, e = s.GetSnapshot(ctx, v.Snapshot.ScenarioID, 2)
	requireCode(t, e, "NOT_FOUND")
	var count int
	s.db.QueryRowContext(ctx, "SELECT count(*) FROM events").Scan(&count)
	if count != 0 {
		t.Fatal("event escaped rollback")
	}
	if _, e = s.db.ExecContext(ctx, "DROP TRIGGER fail_success ON runs"); e != nil {
		t.Fatal(e)
	}
	newPlan, e := s.CommitPlan(ctx, commit)
	if e != nil {
		t.Fatal(e)
	}
	old, e := s.GetScenario(ctx, v.Snapshot.ScenarioID, 1)
	if e != nil || old.Snapshot.Orders[0].Status != c.OrderStatusActive || *old.CurrentPlanID != newPlan.ID {
		t.Fatalf("history %+v %v", old, e)
	}
	r, _ := s.GetRun(ctx, rid)
	if r.Status != "succeeded" {
		t.Fatal(r)
	}
	next, _ := s.GetScenario(ctx, v.Snapshot.ScenarioID, 0)
	_, e = s.Register(ctx, command(next, "blocked"))
	requireCode(t, e, "EVENT_CONFLICT")
	_, e = s.PatchEngineer(ctx, next.Snapshot.ScenarioID, next.Snapshot.Engineers[0].ID, 2, func(*c.Engineer) error { return nil })
	requireCode(t, e, "EVENT_CONFLICT")
	repeated, e := s.Register(ctx, cmd)
	if e != nil || repeated != rid {
		t.Fatalf("event repeat: %v", e)
	}
	other := data.Clone(cmd)
	other.Replan.RequestID = "different"
	_, e = s.Register(ctx, other)
	requireCode(t, e, "IDEMPOTENCY_CONFLICT")
}
func TestCompetingCommitsPatchAndRecovery(t *testing.T) {
	s := testStore(t)
	v, f := seed(t, s)
	ctx := context.Background()
	a, _ := buildCommit(t, s, v, f, "a")
	b, bid := buildCommit(t, s, v, f, "b")
	if _, e := s.CommitPlan(ctx, a); e != nil {
		t.Fatal(e)
	}
	_, e := s.CommitPlan(ctx, b)
	requireCode(t, e, "STALE_VERSION")
	if e = s.Fail(ctx, bid, e); e != nil {
		t.Fatal(e)
	}
	r, _ := s.GetRun(ctx, bid)
	if r.Status != "failed" {
		t.Fatal(r)
	}
	id, e := s.Register(ctx, command(v, "b"))
	if e != nil || id != bid {
		t.Fatal("failed retry changed")
	}
	v, _ = s.GetScenario(ctx, v.Snapshot.ScenarioID, 0)
	updated, e := s.PatchEngineer(ctx, v.Snapshot.ScenarioID, v.Snapshot.Engineers[0].ID, 1, func(eng *c.Engineer) error { eng.Skills = []string{}; return nil })
	if e != nil || updated.Snapshot.Revision != 2 {
		t.Fatal(e)
	}
	old, _ := s.GetSnapshot(ctx, v.Snapshot.ScenarioID, 1)
	if len(old.Engineers[0].Skills) == 0 {
		t.Fatal("mutated history")
	}
	_, e = s.Register(ctx, Command{Kind: "replan", Replan: &c.ReplanRequest{ScenarioID: v.Snapshot.ScenarioID, RequestID: "after-patch", SnapshotRevision: 2, BasePlanID: *v.CurrentPlanID, Event: c.Event{ID: "event"}}})
	requireCode(t, e, "STALE_VERSION")
	_, running := buildCommit(t, s, updated, f, "interrupted")
	if e = s.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	r, _ = s.GetRun(ctx, running)
	if r.Status != "failed" || r.Error == nil {
		t.Fatal(r)
	}
}
func TestStatusFixturesPreserveEquipment(t *testing.T) {
	s := testStore(t)
	v, f := seed(t, s)
	ctx := context.Background()
	in, _ := buildCommit(t, s, v, f, "build")
	p, e := s.CommitPlan(ctx, in)
	if e != nil {
		t.Fatal(e)
	}
	svc := testkit.Plans{Reader: s, Fixture: f}
	for n, step := range f.Execution.Steps {
		req := step.Request
		req.ScenarioID = v.Snapshot.ScenarioID
		req.BasePlanID = p.ID
		req.SnapshotRevision = int64(n + 1)
		if _, e = s.Register(ctx, Command{Kind: "replan", Replan: &req}); e != nil {
			t.Fatal(e)
		}
		s.Claim(ctx)
		result, e := svc.Replan(ctx, req)
		if e != nil {
			t.Fatal(e)
		}
		p, e = s.CommitPlan(ctx, c.PlanCommit{RequestID: req.RequestID, ExpectedRevision: req.SnapshotRevision, ExpectedCurrentPlanID: &req.BasePlanID, Result: result})
		if e != nil {
			t.Fatal(e)
		}
		if n >= 2 && p.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 0 {
			t.Fatal("equipment changed")
		}
	}
	if p.Metrics.CompletedCount != 1 {
		b, _ := json.Marshal(p)
		t.Fatal(string(b))
	}
}
