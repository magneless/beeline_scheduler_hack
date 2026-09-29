package storage

import (
	"context"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestFixedShiftMigrationPreservesHistoryAndExecution(t *testing.T) {
	for _, execution := range []bool{false, true} {
		name := "planned"
		if execution {
			name = "execution recorded"
		}
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			view, fixture := seed(t, s)
			commit, _ := buildCommit(t, s, view, fixture, "initial")
			plan, err := s.CommitPlan(ctx, commit)
			if err != nil {
				t.Fatal(err)
			}
			legacy := view.Snapshot
			legacy.Engineers[0].Shift.Start = legacy.Engineers[0].Shift.Start.Add(-2 * time.Hour)
			legacy.Engineers[0].Shift.End = legacy.Engineers[0].Shift.End.Add(time.Hour)
			reserve := legacy.Engineers[0]
			reserve.ID, reserve.Available, reserve.Reserve = "reserve", false, true
			legacy.Engineers = append(legacy.Engineers, reserve)
			legacy.ReserveInitialized = true
			if _, err = s.db.ExecContext(ctx, "UPDATE snapshots SET body=$2 WHERE scenario_id=$1 AND revision=1", legacy.ScenarioID, encode(legacy)); err != nil {
				t.Fatal(err)
			}
			if execution {
				if _, err = s.db.ExecContext(ctx, `UPDATE snapshots SET body=jsonb_set(body,'{orders,0,execution}','{}') WHERE scenario_id=$1 AND revision=1`, legacy.ScenarioID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = s.db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version='005_fixed_working_shift.sql'"); err != nil {
				t.Fatal(err)
			}
			if err = s.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			if err = s.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetScenario(ctx, legacy.ScenarioID, 0)
			if err != nil {
				t.Fatal(err)
			}
			if execution {
				if got.Snapshot.Revision != 1 || got.CurrentPlanID == nil || *got.CurrentPlanID != plan.ID {
					t.Fatal("migration changed recorded execution")
				}
				if _, err = s.GetSnapshot(ctx, legacy.ScenarioID, 1); err == nil {
					t.Fatal("planning accepted legacy shift")
				}
			} else {
				if got.Snapshot.Revision != 2 || got.CurrentPlanID != nil || got.Snapshot.ReserveInitialized {
					t.Fatalf("expected fresh planning revision: %+v", got)
				}
				if err = c.ValidateEngineerShifts(got.Snapshot); err != nil {
					t.Fatal(err)
				}
				if !got.Snapshot.Engineers[1].Available || got.Snapshot.Engineers[1].Reserve {
					t.Fatal("reserve from obsolete plan was not released")
				}
			}
			history, err := s.GetScenario(ctx, legacy.ScenarioID, 1)
			if err != nil || !history.Snapshot.Engineers[0].Shift.Start.Equal(legacy.Engineers[0].Shift.Start) {
				t.Fatal("historical shift changed")
			}
			oldPlan, err := s.GetPlan(ctx, plan.ID)
			if err != nil || !same(oldPlan, plan) {
				t.Fatal("historical plan changed")
			}
		})
	}
}
