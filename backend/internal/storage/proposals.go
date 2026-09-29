package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// ProposalInput is the complete, immutable input used to calculate a set of options.
type ProposalInput struct {
	ScenarioID            string      `json:"scenario_id"`
	RequestID             string      `json:"request_id"`
	SnapshotRevision      int64       `json:"snapshot_revision"`
	ExpectedCurrentPlanID *string     `json:"expected_current_plan_id"`
	SolveMode             c.SolveMode `json:"solve_mode,omitempty"`
	PendingRevision       *int64      `json:"pending_revision,omitempty"`
	Event                 *c.Event    `json:"event,omitempty"`
}

type Proposal struct {
	ID string `json:"id"`
	ProposalInput
	Options []c.PlanOption `json:"options"`
}

func proposalFromBody(id string, body []byte) (p Proposal, e error) {
	e = json.Unmarshal(body, &p)
	p.ID = id
	return
}

func (s *Store) ProposalByRequest(ctx context.Context, scenarioID, requestID string) (Proposal, bool, error) {
	var id string
	var body []byte
	e := s.db.QueryRowContext(ctx, "SELECT id,body FROM proposals WHERE scenario_id=$1 AND request_id=$2", scenarioID, requestID).Scan(&id, &body)
	if errors.Is(e, sql.ErrNoRows) {
		return Proposal{}, false, nil
	}
	if e != nil {
		return Proposal{}, false, e
	}
	p, e := proposalFromBody(id, body)
	return p, true, e
}

func (s *Store) CurrentProposal(ctx context.Context, scenarioID string) (*Proposal, error) {
	var id string
	var body []byte
	e := s.db.QueryRowContext(ctx, `SELECT p.id,p.body FROM proposals p
		JOIN scenarios s ON s.id=p.scenario_id
		WHERE p.scenario_id=$1 AND p.status='pending' AND p.snapshot_revision=s.revision
		AND p.expected_current_plan_id IS NOT DISTINCT FROM s.current_plan_id
		AND COALESCE((p.body->>'pending_revision')::bigint,0) = COALESCE((SELECT CASE WHEN jsonb_array_length(q.body->'events') > 0 THEN q.revision ELSE 0 END FROM pending_changes q WHERE q.scenario_id=s.id),0)
		ORDER BY p.created_at DESC,p.id DESC LIMIT 1`, scenarioID).Scan(&id, &body)
	if errors.Is(e, sql.ErrNoRows) {
		var exists bool
		if e = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM scenarios WHERE id=$1)", scenarioID).Scan(&exists); e != nil {
			return nil, e
		}
		if !exists {
			return nil, c.NewError("NOT_FOUND", "Сценарий не найден")
		}
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	p, e := proposalFromBody(id, body)
	return &p, e
}

func (s *Store) SaveProposal(ctx context.Context, in ProposalInput, options []c.PlanOption) (Proposal, error) {
	var out Proposal
	if strings.TrimSpace(in.ScenarioID) == "" || strings.TrimSpace(in.RequestID) == "" || in.SnapshotRevision < 1 || len(options) == 0 {
		return out, c.NewError("INVALID_INPUT", "Некорректный запрос или пустой набор вариантов")
	}
	seen := map[string]bool{}
	for _, option := range options {
		if option.Key == "" || seen[option.Key] {
			return out, c.NewError("INVALID_INPUT", "Ключи вариантов должны быть уникальными")
		}
		seen[option.Key] = true
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	snap, currentPlanID, e := current(ctx, tx, in.ScenarioID)
	if e != nil {
		return out, e
	}
	var id string
	var body []byte
	e = tx.QueryRowContext(ctx, "SELECT id,body FROM proposals WHERE scenario_id=$1 AND request_id=$2", in.ScenarioID, in.RequestID).Scan(&id, &body)
	if e == nil {
		old, err := proposalFromBody(id, body)
		if err != nil {
			return out, err
		}
		if !same(old.ProposalInput, in) {
			return out, c.NewError("IDEMPOTENCY_CONFLICT", "Ключ запроса уже использован")
		}
		return old, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	var used bool
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM runs WHERE scenario_id=$1 AND request_id=$2)", in.ScenarioID, in.RequestID).Scan(&used); e != nil {
		return out, e
	}
	if used {
		return out, c.NewError("IDEMPOTENCY_CONFLICT", "Ключ запроса уже использован")
	}
	if snap.Revision != in.SnapshotRevision || !ptrEqual(currentPlanID, in.ExpectedCurrentPlanID) {
		return out, c.NewError("STALE_VERSION", "Исходные данные или текущий план изменились")
	}
	if e = checkPending(ctx, tx, in.ScenarioID, in.PendingRevision); e != nil {
		return out, e
	}
	if in.Event == nil {
		if e = blocked(ctx, tx, snap, currentPlanID); e != nil {
			return out, e
		}
	} else {
		if currentPlanID == nil {
			return out, c.NewError("EVENT_CONFLICT", "Для события нужен действующий план")
		}
		var planRevision int64
		if e = tx.QueryRowContext(ctx, "SELECT revision FROM plans WHERE id=$1 AND scenario_id=$2", *currentPlanID, in.ScenarioID).Scan(&planRevision); e != nil {
			return out, missing(e)
		}
		if planRevision != snap.Revision {
			return out, c.NewError("STALE_VERSION", "План устарел после изменения данных")
		}
		if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM events WHERE scenario_id=$1 AND id=$2)", in.ScenarioID, in.Event.ID).Scan(&used); e != nil {
			return out, e
		}
		if used {
			return out, c.NewError("IDEMPOTENCY_CONFLICT", "Событие уже применено")
		}
	}
	out = Proposal{ID: ID("proposal"), ProposalInput: in, Options: options}
	var eventID *string
	if in.Event != nil {
		eventID = &in.Event.ID
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO proposals(id,scenario_id,request_id,snapshot_revision,expected_current_plan_id,event_id,body)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, out.ID, in.ScenarioID, in.RequestID, in.SnapshotRevision, in.ExpectedCurrentPlanID, eventID, encode(out))
	if e != nil {
		return Proposal{}, e
	}
	return out, tx.Commit()
}

// AcceptProposal commits exactly the selected, previously stored result.
// Locking the scenario serializes this operation with patches and legacy commits.
func (s *Store) AcceptProposal(ctx context.Context, proposalID, requestID, optionKey string) (c.Plan, error) {
	var plan c.Plan
	if proposalID == "" || strings.TrimSpace(requestID) == "" || strings.TrimSpace(optionKey) == "" {
		return plan, c.NewError("INVALID_INPUT", "Требуются proposal_id, request_id и option_key")
	}
	var scenarioID string
	e := s.db.QueryRowContext(ctx, "SELECT scenario_id FROM proposals WHERE id=$1", proposalID).Scan(&scenarioID)
	if e != nil {
		return plan, missing(e)
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return plan, e
	}
	defer tx.Rollback()
	snap, currentPlanID, e := current(ctx, tx, scenarioID)
	if e != nil {
		return plan, e
	}
	var status string
	var acceptedOption, acceptedRequest sql.NullString
	var acceptedPlanID *string
	var body []byte
	e = tx.QueryRowContext(ctx, `SELECT body,status,accepted_option_key,accepted_request_id,accepted_plan_id
		FROM proposals WHERE id=$1 AND scenario_id=$2 FOR UPDATE`, proposalID, scenarioID).
		Scan(&body, &status, &acceptedOption, &acceptedRequest, &acceptedPlanID)
	if e != nil {
		return plan, missing(e)
	}
	proposal, e := proposalFromBody(proposalID, body)
	if e != nil {
		return plan, e
	}
	if status == "accepted" {
		if acceptedRequest.String != requestID {
			return plan, c.NewError("STALE_VERSION", "Другой вариант этого предложения уже принят")
		}
		if acceptedOption.String != optionKey {
			return plan, c.NewError("IDEMPOTENCY_CONFLICT", "Предложение уже принято с другим вариантом")
		}
		if acceptedPlanID == nil {
			return plan, c.NewError("EVENT_CONFLICT", "Отсутствует принятый план")
		}
		var saved []byte
		if e = tx.QueryRowContext(ctx, "SELECT body FROM plans WHERE id=$1", *acceptedPlanID).Scan(&saved); e != nil {
			return plan, missing(e)
		}
		e = json.Unmarshal(saved, &plan)
		return plan, e
	}
	if snap.Revision != proposal.SnapshotRevision || !ptrEqual(currentPlanID, proposal.ExpectedCurrentPlanID) {
		return plan, c.NewError("STALE_VERSION", "Исходные данные или план изменились после расчёта")
	}
	if e = checkPending(ctx, tx, scenarioID, proposal.PendingRevision); e != nil {
		return plan, e
	}
	var reused bool
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM proposals WHERE scenario_id=$1 AND accepted_request_id=$2)
		OR EXISTS(SELECT 1 FROM runs WHERE scenario_id=$1 AND request_id=$2)`, scenarioID, requestID).Scan(&reused); e != nil {
		return plan, e
	}
	if reused {
		return plan, c.NewError("IDEMPOTENCY_CONFLICT", "Ключ принятия уже использован")
	}
	var selected *c.PlanOption
	for i := range proposal.Options {
		if proposal.Options[i].Key == optionKey {
			selected = &proposal.Options[i]
			break
		}
	}
	if selected == nil {
		return plan, c.NewError("INVALID_INPUT", "Вариант не найден в предложении")
	}
	if optionKey == "original" && proposal.Event != nil && containsUnavailable(*proposal.Event) {
		return plan, c.NewError("INVALID_INPUT", "Исходный маршрут недоступен: инженер не работает")
	}
	result := selected.Result
	draft := result.Draft
	target := result.TargetSnapshot
	invalidPlan := func() (c.Plan, error) {
		return plan, c.NewError("INVALID_PLAN", "Рассчитанный вариант не соответствует сценарию")
	}
	if c.ValidateEngineerShifts(target) != nil {
		return invalidPlan()
	}
	if draft.ScenarioID != scenarioID || target.ScenarioID != scenarioID || target.RegionID != snap.RegionID || target.Date != snap.Date || target.Timezone != snap.Timezone || target.OfficeLocationID != snap.OfficeLocationID {
		return invalidPlan()
	}
	if proposal.Event == nil {
		if e = blocked(ctx, tx, snap, currentPlanID); e != nil {
			return plan, e
		}
		if result.AppliedEvent != nil || draft.BasePlanID != nil || draft.SnapshotRevision != snap.Revision || !same(target, snap) {
			return invalidPlan()
		}
		zone, err := time.LoadLocation(snap.Timezone)
		if err != nil {
			return plan, err
		}
		day, err := time.ParseInLocation("2006-01-02", snap.Date, zone)
		if err != nil || !draft.AsOf.Equal(day) {
			return invalidPlan()
		}
	} else {
		ev := proposal.Event
		if currentPlanID == nil || draft.BasePlanID == nil || *draft.BasePlanID != *currentPlanID || draft.SnapshotRevision != snap.Revision+1 || target.Revision != snap.Revision+1 || result.AppliedEvent == nil {
			return invalidPlan()
		}
		expectedAsOf, err := eventPlanTime(ctx, tx, *currentPlanID, ev.OccurredAt)
		if err != nil {
			return plan, err
		}
		if !draft.AsOf.Equal(expectedAsOf) {
			return invalidPlan()
		}
		if result.AppliedEvent.ID != ev.ID || result.AppliedEvent.Type != ev.Type || !result.AppliedEvent.OccurredAt.Equal(ev.OccurredAt) {
			return invalidPlan()
		}
		if ev.Type != "urgent_order_added" && ev.Type != "ordinary_order_added" && !same(result.AppliedEvent.Payload, ev.Payload) {
			return invalidPlan()
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,$2,$3)", scenarioID, target.Revision, encode(target)); e != nil {
			return plan, e
		}
	}
	if proposal.Event == nil && currentPlanID == nil {
		var changed bool
		target, changed = initialReserve(target, draft.Routes)
		if changed {
			draft.SnapshotRevision = target.Revision
			if _, e = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,$2,$3)", scenarioID, target.Revision, encode(target)); e != nil {
				return plan, e
			}
		}
	}
	plan = c.Plan{ID: ID("plan"), PlanDraft: draft}
	if _, e = tx.ExecContext(ctx, "INSERT INTO plans VALUES($1,$2,$3,$4)", plan.ID, scenarioID, draft.SnapshotRevision, encode(plan)); e != nil {
		return plan, e
	}
	if proposal.PendingRevision != nil {
		q, err := readPending(ctx, tx, scenarioID)
		if err != nil {
			return plan, err
		}
		for _, ev := range q.Events {
			if _, e = tx.ExecContext(ctx, "INSERT INTO events VALUES($1,$2,$3,$4)", scenarioID, ev.ID, plan.ID, encode(ev)); e != nil {
				return plan, e
			}
		}
		q.Revision++
		q.Events, q.Snapshot = []c.Event{}, nil
		if _, e = tx.ExecContext(ctx, "UPDATE pending_changes SET revision=$2,body=$3 WHERE scenario_id=$1", scenarioID, q.Revision, encode(q)); e != nil {
			return plan, e
		}
	}
	if result.AppliedEvent != nil {
		if _, e = tx.ExecContext(ctx, "INSERT INTO events VALUES($1,$2,$3,$4)", scenarioID, result.AppliedEvent.ID, plan.ID, encode(result.AppliedEvent)); e != nil {
			return plan, e
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE scenarios SET revision=$2,current_plan_id=$3 WHERE id=$1", scenarioID, target.Revision, plan.ID); e != nil {
		return plan, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE proposals SET status='accepted',accepted_option_key=$2,
		accepted_request_id=$3,accepted_plan_id=$4 WHERE id=$1`, proposalID, optionKey, requestID, plan.ID); e != nil {
		return plan, e
	}
	return plan, tx.Commit()
}

func containsUnavailable(ev c.Event) bool {
	if ev.Type == "engineer_unavailable" {
		return true
	}
	if ev.Type != "pending_changes" {
		return false
	}
	var payload struct {
		Events []c.Event `json:"events"`
	}
	_ = json.Unmarshal(ev.Payload, &payload)
	for _, item := range payload.Events {
		if item.Type == "engineer_unavailable" {
			return true
		}
	}
	return false
}
