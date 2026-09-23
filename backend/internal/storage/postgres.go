package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct{ db *sql.DB }

func ID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
func Open(ctx context.Context, url string) (*Store, error) {
	db, e := sql.Open("pgx", url)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(10)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Migrate(ctx context.Context) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(73124011)"); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY)"); e != nil {
		return e
	}
	entries, e := migrations.ReadDir("migrations")
	if e != nil {
		return e
	}
	for _, f := range entries {
		var exists bool
		if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", f.Name()).Scan(&exists); e != nil {
			return e
		}
		if exists {
			continue
		}
		b, e := migrations.ReadFile("migrations/" + f.Name())
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, string(b)); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES($1)", f.Name()); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func missing(e error) error {
	if errors.Is(e, sql.ErrNoRows) {
		return c.NewError("NOT_FOUND", "Объект или ревизия не найдены")
	}
	return e
}
func encode(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return string(b)
}
func same(a, b any) bool {
	var x, y any
	da := json.NewDecoder(bytes.NewBufferString(encode(a)))
	da.UseNumber()
	da.Decode(&x)
	db := json.NewDecoder(bytes.NewBufferString(encode(b)))
	db.UseNumber()
	db.Decode(&y)
	return reflect.DeepEqual(x, y)
}
func ptrEqual(a, b *string) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
func (s *Store) CreateScenario(ctx context.Context, snap c.Snapshot, metadata any) (c.ScenarioView, error) {
	if _, err := json.Marshal(snap); err != nil {
		return c.ScenarioView{}, c.NewError("INVALID_INPUT", "Снимок содержит некорректные значения")
	}
	if _, err := json.Marshal(metadata); err != nil {
		return c.ScenarioView{}, err
	}
	snap.Revision = 1
	if snap.ScenarioID == "" {
		snap.ScenarioID = ID("scenario")
	}
	v := c.ScenarioView{Snapshot: snap}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "INSERT INTO scenarios(id,revision) VALUES($1,1)", snap.ScenarioID); e != nil {
		return v, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,1,$2)", snap.ScenarioID, encode(snap)); e != nil {
		return v, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO imports VALUES($1,$2)", snap.ScenarioID, encode(metadata)); e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func (s *Store) GetSnapshot(ctx context.Context, id string, rev int64) (v c.Snapshot, e error) {
	var b []byte
	e = s.db.QueryRowContext(ctx, "SELECT body FROM snapshots WHERE scenario_id=$1 AND revision=$2", id, rev).Scan(&b)
	if e != nil {
		return v, missing(e)
	}
	e = json.Unmarshal(b, &v)
	return
}
func (s *Store) GetScenario(ctx context.Context, id string, rev int64) (v c.ScenarioView, e error) {
	var b []byte
	e = s.db.QueryRowContext(ctx, "SELECT p.body,s.current_plan_id FROM scenarios s JOIN snapshots p ON p.scenario_id=s.id AND p.revision=CASE WHEN $2::bigint=0 THEN s.revision ELSE $2 END WHERE s.id=$1", id, rev).Scan(&b, &v.CurrentPlanID)
	if e != nil {
		return v, missing(e)
	}
	e = json.Unmarshal(b, &v.Snapshot)
	return
}
func (s *Store) GetPlan(ctx context.Context, id string) (p c.Plan, e error) {
	var b []byte
	e = s.db.QueryRowContext(ctx, "SELECT body FROM plans WHERE id=$1", id).Scan(&b)
	if e != nil {
		return p, missing(e)
	}
	e = json.Unmarshal(b, &p)
	return
}

type Command struct {
	Kind   string              `json:"kind"`
	Build  *c.BuildPlanRequest `json:"build"`
	Replan *c.ReplanRequest    `json:"replan"`
}

func (cmd Command) fields() (scenario, request string, revision int64, plan *string, event *string) {
	if cmd.Build != nil {
		b := cmd.Build
		return b.ScenarioID, b.RequestID, b.SnapshotRevision, b.ExpectedCurrentPlanID, nil
	}
	r := cmd.Replan
	return r.ScenarioID, r.RequestID, r.SnapshotRevision, &r.BasePlanID, &r.Event.ID
}
func current(ctx context.Context, tx *sql.Tx, id string) (snap c.Snapshot, plan *string, e error) {
	var rev int64
	var b []byte
	e = tx.QueryRowContext(ctx, "SELECT revision,current_plan_id FROM scenarios WHERE id=$1 FOR UPDATE", id).Scan(&rev, &plan)
	if e != nil {
		return snap, plan, missing(e)
	}
	e = tx.QueryRowContext(ctx, "SELECT body FROM snapshots WHERE scenario_id=$1 AND revision=$2", id, rev).Scan(&b)
	if e == nil {
		e = json.Unmarshal(b, &snap)
	}
	return
}
func blocked(ctx context.Context, tx *sql.Tx, snap c.Snapshot, pid *string) error {
	for _, o := range snap.Orders {
		if o.Execution != nil {
			return c.NewError("EVENT_CONFLICT", "Изменения дня выполняются через события")
		}
	}
	if pid == nil {
		return nil
	}
	var b []byte
	if e := tx.QueryRowContext(ctx, "SELECT body FROM plans WHERE id=$1", *pid).Scan(&b); e != nil {
		return e
	}
	var p c.Plan
	if e := json.Unmarshal(b, &p); e != nil {
		return e
	}
	z, e := time.LoadLocation(snap.Timezone)
	if e != nil {
		return e
	}
	day, e := time.ParseInLocation("2006-01-02", snap.Date, z)
	if e != nil {
		return e
	}
	if p.BasePlanID != nil || p.AsOf.After(day) {
		return c.NewError("EVENT_CONFLICT", "После первого события доступны только события")
	}
	return nil
}
func (s *Store) Register(ctx context.Context, cmd Command) (string, error) {
	if (cmd.Kind == "build" && (cmd.Build == nil || cmd.Replan != nil)) || (cmd.Kind == "replan" && (cmd.Replan == nil || cmd.Build != nil)) || (cmd.Kind != "build" && cmd.Kind != "replan") {
		return "", c.NewError("INVALID_INPUT", "Некорректная команда")
	}
	sid, rid, rev, pid, eid := cmd.fields()
	if sid == "" || rid == "" || rev < 1 || (eid != nil && *eid == "") {
		return "", c.NewError("INVALID_INPUT", "Некорректные ключи или ревизия команды")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	snap, cur, e := current(ctx, tx, sid)
	if e != nil {
		return "", e
	}
	rows, e := tx.QueryContext(ctx, "SELECT id,command FROM runs WHERE scenario_id=$1 AND (request_id=$2 OR event_id=$3)", sid, rid, eid)
	if e != nil {
		return "", e
	}
	existing := ""
	conflict := false
	for rows.Next() {
		var id string
		var b []byte
		if e = rows.Scan(&id, &b); e != nil {
			rows.Close()
			return "", e
		}
		var old Command
		if e = json.Unmarshal(b, &old); e != nil {
			rows.Close()
			return "", e
		}
		if !same(old, cmd) {
			conflict = true
		}
		existing = id
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return "", e
	}
	if conflict {
		return "", c.NewError("IDEMPOTENCY_CONFLICT", "Ключ уже использован с другим содержимым")
	}
	if existing != "" {
		return existing, nil
	}
	if snap.Revision != rev || !ptrEqual(cur, pid) {
		return "", c.NewError("STALE_VERSION", "Исходные данные или текущий план изменились")
	}
	if cmd.Kind == "build" {
		if e = blocked(ctx, tx, snap, cur); e != nil {
			return "", e
		}
	} else {
		var pr int64
		if e = tx.QueryRowContext(ctx, "SELECT revision FROM plans WHERE id=$1", pid).Scan(&pr); e != nil {
			return "", missing(e)
		}
		if pr != rev {
			return "", c.NewError("STALE_VERSION", "После редактирования необходим новый Build")
		}
	}
	id := ID("run")
	_, e = tx.ExecContext(ctx, "INSERT INTO runs(id,scenario_id,request_id,event_id,command,status) VALUES($1,$2,$3,$4,$5,'queued')", id, sid, rid, eid, encode(cmd))
	if e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) GetRun(ctx context.Context, id string) (r c.Run, e error) {
	var b []byte
	e = s.db.QueryRowContext(ctx, "SELECT id,scenario_id,status,plan_id,error FROM runs WHERE id=$1", id).Scan(&r.ID, &r.ScenarioID, &r.Status, &r.PlanID, &b)
	if e != nil {
		return r, missing(e)
	}
	if b != nil {
		e = json.Unmarshal(b, &r.Error)
	}
	return
}
func (s *Store) Claim(ctx context.Context) (id string, cmd Command, e error) {
	var b []byte
	e = s.db.QueryRowContext(ctx, "UPDATE runs SET status='running' WHERE id=(SELECT id FROM runs WHERE status='queued' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,command").Scan(&id, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return "", cmd, nil
	}
	if e == nil {
		e = json.Unmarshal(b, &cmd)
	}
	return
}
func (s *Store) Fail(ctx context.Context, id string, err error) error {
	var ce *c.ContractError
	if !errors.As(err, &ce) {
		ce = c.NewError("COMPUTATION_FAILED", "Ошибка выполнения расчёта")
	}
	if ce.Details == nil {
		copy := *ce
		copy.Details = map[string]any{}
		ce = &copy
	}
	_, e := s.db.ExecContext(ctx, "UPDATE runs SET status='failed',error=$2 WHERE id=$1 AND status IN ('queued','running')", id, encode(ce))
	return e
}
func (s *Store) Recover(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, "UPDATE runs SET status='failed',error=$1 WHERE status='running'", encode(c.NewError("COMPUTATION_FAILED", "Запуск прерван перезапуском сервера; отправьте новую команду")))
	return e
}
func (s *Store) CommitPlan(ctx context.Context, in c.PlanCommit) (c.Plan, error) {
	var p c.Plan
	if _, err := json.Marshal(in); err != nil {
		return p, c.NewError("INVALID_PLAN", "Результат содержит несериализуемые значения")
	}
	sid := in.Result.Draft.ScenarioID
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return p, e
	}
	defer tx.Rollback()
	snap, cur, e := current(ctx, tx, sid)
	if e != nil {
		return p, e
	}
	var status, runID string
	var saved, cmdBody []byte
	var planID *string
	e = tx.QueryRowContext(ctx, "SELECT id,status,plan_id,commit_input,command FROM runs WHERE scenario_id=$1 AND request_id=$2 FOR UPDATE", sid, in.RequestID).Scan(&runID, &status, &planID, &saved, &cmdBody)
	if e != nil {
		return p, missing(e)
	}
	if status == "succeeded" {
		var old c.PlanCommit
		if e = json.Unmarshal(saved, &old); e != nil {
			return p, e
		}
		if !same(old, in) {
			return p, c.NewError("IDEMPOTENCY_CONFLICT", "Результат повторного сохранения отличается")
		}
		return s.GetPlan(ctx, *planID)
	}
	if status != "running" {
		return p, c.NewError("EVENT_CONFLICT", "Запуск не выполняется")
	}
	var cmd Command
	if e = json.Unmarshal(cmdBody, &cmd); e != nil {
		return p, e
	}
	_, _, rev, pid, _ := cmd.fields()
	if in.ExpectedRevision != rev || !ptrEqual(in.ExpectedCurrentPlanID, pid) {
		return p, c.NewError("INVALID_PLAN", "Ожидания сохранения не совпадают с командой")
	}
	if snap.Revision != rev || !ptrEqual(cur, pid) {
		return p, c.NewError("STALE_VERSION", "Исходные данные или план изменились во время расчёта")
	}
	result := in.Result
	draft := result.Draft
	target := result.TargetSnapshot
	invalid := func() (c.Plan, error) {
		return p, c.NewError("INVALID_PLAN", "Несогласованные снимок, событие и план")
	}
	if target.ScenarioID != sid || target.RegionID != snap.RegionID || target.Date != snap.Date || target.Timezone != snap.Timezone {
		return invalid()
	}
	if target.OfficeLocationID != snap.OfficeLocationID {
		return invalid()
	}
	if cmd.Kind == "build" {
		if e = blocked(ctx, tx, snap, cur); e != nil {
			return p, e
		}
		if result.AppliedEvent != nil || draft.BasePlanID != nil || draft.SnapshotRevision != rev || !same(target, snap) {
			return invalid()
		}
		zone, err := time.LoadLocation(snap.Timezone)
		if err != nil {
			return p, err
		}
		day, err := time.ParseInLocation("2006-01-02", snap.Date, zone)
		if err != nil {
			return p, err
		}
		if !draft.AsOf.Equal(day) {
			return invalid()
		}
	} else {
		if result.AppliedEvent == nil || result.AppliedEvent.ID != cmd.Replan.Event.ID || result.AppliedEvent.Type != cmd.Replan.Event.Type || !result.AppliedEvent.OccurredAt.Equal(cmd.Replan.Event.OccurredAt) || !ptrEqual(draft.BasePlanID, pid) || target.Revision != rev+1 || draft.SnapshotRevision != rev+1 {
			return invalid()
		}
		if !draft.AsOf.Equal(cmd.Replan.Event.OccurredAt) {
			return invalid()
		}
		if cmd.Replan.Event.Type != "urgent_order_added" && !same(result.AppliedEvent.Payload, cmd.Replan.Event.Payload) {
			return invalid()
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,$2,$3)", sid, target.Revision, encode(target)); e != nil {
			return p, e
		}
	}
	p = c.Plan{ID: ID("plan"), PlanDraft: draft}
	if _, e = tx.ExecContext(ctx, "INSERT INTO plans VALUES($1,$2,$3,$4)", p.ID, sid, draft.SnapshotRevision, encode(p)); e != nil {
		return p, e
	}
	if result.AppliedEvent != nil {
		if _, e = tx.ExecContext(ctx, "INSERT INTO events VALUES($1,$2,$3,$4)", sid, result.AppliedEvent.ID, p.ID, encode(result.AppliedEvent)); e != nil {
			return p, e
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE scenarios SET revision=$2,current_plan_id=$3 WHERE id=$1", sid, target.Revision, p.ID); e != nil {
		return p, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE runs SET status='succeeded',plan_id=$2,commit_input=$3 WHERE id=$1", runID, p.ID, encode(in)); e != nil {
		return p, e
	}
	return p, tx.Commit()
}
func (s *Store) PatchEngineer(ctx context.Context, id, eid string, rev int64, apply func(*c.Engineer) error) (c.ScenarioView, error) {
	var v c.ScenarioView
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	snap, pid, e := current(ctx, tx, id)
	if e != nil {
		return v, e
	}
	if snap.Revision != rev {
		return v, c.NewError("STALE_VERSION", "Ревизия изменилась")
	}
	if e = blocked(ctx, tx, snap, pid); e != nil {
		return v, e
	}
	found := false
	for i := range snap.Engineers {
		if snap.Engineers[i].ID == eid {
			found = true
			if e = apply(&snap.Engineers[i]); e != nil {
				return v, e
			}
			break
		}
	}
	if !found {
		return v, c.NewError("NOT_FOUND", "Инженер не найден")
	}
	snap.Revision++
	if _, e = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,$2,$3)", id, snap.Revision, encode(snap)); e != nil {
		return v, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE scenarios SET revision=$2 WHERE id=$1", id, snap.Revision); e != nil {
		return v, e
	}
	v = c.ScenarioView{Snapshot: snap, CurrentPlanID: pid}
	return v, tx.Commit()
}

var _ c.DataStore = (*Store)(nil)

// AcquireWorker prevents a second process from resetting another process's runs.
func (s *Store) AcquireWorker(ctx context.Context) (func(), error) {
	conn, e := s.db.Conn(ctx)
	if e != nil {
		return nil, e
	}
	var ok bool
	if e = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(73124012)").Scan(&ok); e != nil {
		conn.Close()
		return nil, e
	}
	if !ok {
		conn.Close()
		return nil, c.NewError("EVENT_CONFLICT", "Другой сервер уже выполняет запуски")
	}
	return func() {
		release, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		conn.ExecContext(release, "SELECT pg_advisory_unlock(73124012)")
		conn.Close()
	}, nil
}
