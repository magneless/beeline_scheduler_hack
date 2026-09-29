package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

type PendingChanges struct {
	ScenarioID       string      `json:"scenario_id"`
	Revision         int64       `json:"revision"`
	SnapshotRevision int64       `json:"snapshot_revision"`
	BasePlanID       *string     `json:"base_plan_id"`
	Events           []c.Event   `json:"events"`
	Snapshot         *c.Snapshot `json:"snapshot,omitempty"`
}

func readPending(ctx context.Context, tx *sql.Tx, sid string) (PendingChanges, error) {
	q := PendingChanges{ScenarioID: sid, Events: []c.Event{}}
	var body []byte
	err := tx.QueryRowContext(ctx, "SELECT body FROM pending_changes WHERE scenario_id=$1", sid).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return q, nil
	}
	if err == nil {
		err = json.Unmarshal(body, &q)
	}
	return q, err
}

func (s *Store) Pending(ctx context.Context, sid string) (PendingChanges, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PendingChanges{}, err
	}
	defer tx.Rollback()
	snap, pid, err := current(ctx, tx, sid)
	if err != nil {
		return PendingChanges{}, err
	}
	q, err := readPending(ctx, tx, sid)
	if len(q.Events) == 0 {
		q.SnapshotRevision, q.BasePlanID, q.Snapshot = snap.Revision, pid, nil
	}
	return q, err
}

// SavePending serializes with plan acceptance, direct events and data edits.
// Validation runs outside this transaction; both revisions are checked again.
func (s *Store) SavePending(ctx context.Context, q PendingChanges, expected int64) (PendingChanges, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return q, err
	}
	defer tx.Rollback()
	snap, pid, err := current(ctx, tx, q.ScenarioID)
	if err != nil {
		return q, err
	}
	old, err := readPending(ctx, tx, q.ScenarioID)
	if err != nil {
		return q, err
	}
	if old.Revision != expected || snap.Revision != q.SnapshotRevision || !ptrEqual(pid, q.BasePlanID) {
		return q, c.NewError("STALE_VERSION", "Очередь изменений или план уже изменились. Обновите смену.")
	}
	var running bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM runs WHERE scenario_id=$1 AND status IN ('queued','running'))", q.ScenarioID).Scan(&running); err != nil {
		return q, err
	}
	if running {
		return q, c.NewError("EVENT_CONFLICT", "Дождитесь сохранения предыдущего события")
	}
	for _, event := range q.Events {
		var used bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM events WHERE scenario_id=$1 AND id=$2)", q.ScenarioID, event.ID).Scan(&used); err != nil {
			return q, err
		}
		if used {
			return q, c.NewError("IDEMPOTENCY_CONFLICT", "Событие уже применено")
		}
	}
	q.Revision = expected + 1
	_, err = tx.ExecContext(ctx, `INSERT INTO pending_changes VALUES($1,$2,$3)
		ON CONFLICT(scenario_id) DO UPDATE SET revision=excluded.revision,body=excluded.body`, q.ScenarioID, q.Revision, encode(q))
	if err != nil {
		return q, err
	}
	return q, tx.Commit()
}

func checkPending(ctx context.Context, tx *sql.Tx, sid string, expected *int64) error {
	q, err := readPending(ctx, tx, sid)
	if err != nil {
		return err
	}
	if expected == nil && len(q.Events) > 0 || expected != nil && (*expected != q.Revision || len(q.Events) == 0) {
		return c.NewError("STALE_VERSION", "Изменилась очередь событий. Рассчитайте варианты заново.")
	}
	return nil
}
