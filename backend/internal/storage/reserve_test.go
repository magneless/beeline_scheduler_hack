package storage

import (
	"context"
	"reflect"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"testing"
)

func TestInitialReserveUsesOnlyOriginalUnassignedCrews(t *testing.T) {
	snap := c.Snapshot{Revision: 1, Engineers: []c.Engineer{
		{ID: "busy", Available: true}, {ID: "idle", Available: true}, {ID: "sick", Available: false},
	}}
	target, changed := initialReserve(snap, []c.Route{{EngineerID: "busy", Visits: []c.Visit{{OrderID: "job"}}}})
	if !changed || target.Revision != 2 || len(target.Engineers) != 3 {
		t.Fatalf("unexpected reserve snapshot: %+v", target)
	}
	if target.Engineers[0].Reserve || !target.Engineers[0].Available || !target.Engineers[1].Reserve || target.Engineers[1].Available || target.Engineers[2].Reserve {
		t.Fatalf("wrong pool: %+v", target.Engineers)
	}
	if snap.Engineers[1].Reserve || !snap.Engineers[1].Available {
		t.Fatal("preview source mutated")
	}
}

func TestLegacyDemoReserveMigrationPreservesRoutesAndHistory(t *testing.T) {
	s := testStore(t)
	view, fixture := seed(t, s)
	ctx := context.Background()
	snap := view.Snapshot
	idle := snap.Engineers[0]
	idle.ID = "original-idle"
	idle.SourceOrder = 2
	extra := idle
	extra.ID = snap.RegionID + "-eng-09"
	extra.SourceOrder = 9
	extra.Available = false
	extra.Reserve = true
	snap.Engineers = append(snap.Engineers, idle, extra)
	snap.Issues = append(snap.Issues, c.Issue{Code: "DEMO_ENGINEERS", Message: "Legacy demo"})
	if _, err := s.db.ExecContext(ctx, "UPDATE snapshots SET body=$3 WHERE scenario_id=$1 AND revision=$2", snap.ScenarioID, snap.Revision, encode(snap)); err != nil {
		t.Fatal(err)
	}
	plan := c.Plan{ID: "legacy-root", PlanDraft: fixture.PlanResult.Draft}
	plan.ScenarioID = snap.ScenarioID
	if _, err := s.db.ExecContext(ctx, "INSERT INTO plans VALUES($1,$2,$3,$4)", plan.ID, snap.ScenarioID, snap.Revision, encode(plan)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE scenarios SET current_plan_id=$2 WHERE id=$1", snap.ScenarioID, plan.ID); err != nil {
		t.Fatal(err)
	}
	sql, err := migrations.ReadFile("migrations/003_derived_demo_reserve.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = s.db.ExecContext(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	current, err := s.GetScenario(ctx, snap.ScenarioID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if current.Snapshot.Revision != 2 || len(current.Snapshot.Engineers) != 2 || !current.Snapshot.Engineers[1].Reserve || current.Snapshot.Engineers[1].Available {
		t.Fatalf("wrong migration: %+v", current)
	}
	updated, err := s.GetPlan(ctx, *current.CurrentPlanID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated.Routes, plan.Routes) || !reflect.DeepEqual(updated.Metrics, plan.Metrics) || !updated.AsOf.Equal(plan.AsOf) {
		t.Fatal("working routes changed during migration")
	}
	old, err := s.GetSnapshot(ctx, snap.ScenarioID, 1)
	if err != nil || len(old.Engineers) != 3 {
		t.Fatal("snapshot history lost")
	}
	oldPlan, err := s.GetPlan(ctx, plan.ID)
	if err != nil || !reflect.DeepEqual(oldPlan, plan) {
		t.Fatal("plan history changed")
	}
}
