package storage

import (
	"context"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// ReplaceEngineers atomically replaces a scenario roster before execution starts.
func (s *Store) ReplaceEngineers(ctx context.Context, id string, rev int64, engineers []c.Engineer) (c.ScenarioView, error) {
	var v c.ScenarioView
	if len(engineers) == 0 {
		return v, c.NewError("INVALID_INPUT", "Состав инженеров не может быть пустым")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	snap, pid, err := current(ctx, tx, id)
	if err != nil {
		return v, err
	}
	if snap.Revision != rev {
		return v, c.NewError("STALE_VERSION", "Ревизия изменилась")
	}
	if err = blocked(ctx, tx, snap, pid); err != nil {
		return v, err
	}
	snap.Engineers = append([]c.Engineer(nil), engineers...)
	if err = c.ValidateEngineerShifts(snap); err != nil {
		return v, err
	}
	filtered := snap.Issues[:0]
	for _, issue := range snap.Issues {
		if issue.Code != "ENGINEERS_REQUIRED" && issue.Code != "DEMO_ENGINEERS" {
			filtered = append(filtered, issue)
		}
	}
	snap.Issues = filtered
	snap.Revision++
	if _, err = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,$2,$3)", id, snap.Revision, encode(snap)); err != nil {
		return v, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE scenarios SET revision=$2,current_plan_id=NULL WHERE id=$1", id, snap.Revision); err != nil {
		return v, err
	}
	v = c.ScenarioView{Snapshot: snap, CurrentPlanID: nil}
	return v, tx.Commit()
}
