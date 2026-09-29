package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

type optionService interface {
	BuildOptions(ctx context.Context, in c.BuildPlanRequest) ([]c.PlanOption, error)
	ReplanOptions(ctx context.Context, in c.ReplanRequest) ([]c.PlanOption, error)
}

func (s *Server) createProposal(w http.ResponseWriter, r *http.Request) {
	service, ok := s.Plans.(optionService)
	if !ok {
		failure(w, c.NewError("COMPUTATION_FAILED", "Сервис вариантов плана недоступен"))
		return
	}
	var in struct {
		RequestID       string          `json:"request_id"`
		Revision        int64           `json:"snapshot_revision"`
		Expected        json.RawMessage `json:"expected_current_plan_id"`
		SolveMode       c.SolveMode     `json:"solve_mode"`
		PendingRevision *int64          `json:"pending_revision"`
		Event           *c.Event        `json:"event"`
	}
	if e := decode(r, &in); e != nil {
		failure(w, e)
		return
	}
	if strings.TrimSpace(in.RequestID) == "" || in.Revision < 1 || len(in.Expected) == 0 {
		failure(w, invalid("Требуются request_id, snapshot_revision и expected_current_plan_id"))
		return
	}
	if in.SolveMode != "" && in.SolveMode != c.SolveModeBaseline && in.SolveMode != c.SolveModeOptimized {
		failure(w, invalid("solve_mode должен быть baseline или optimized"))
		return
	}
	var expected *string
	if e := json.Unmarshal(in.Expected, &expected); e != nil || (expected != nil && strings.TrimSpace(*expected) == "") {
		failure(w, invalid("Некорректный expected_current_plan_id"))
		return
	}
	if in.Event != nil {
		if expected == nil {
			failure(w, invalid("Для события нужен действующий план"))
			return
		}
		if e := validateEvent(*in.Event); e != nil {
			failure(w, e)
			return
		}
		if in.Event.Type == "order_status_changed" {
			failure(w, invalid("Начало и завершение работы фиксируются через события плана"))
			return
		}
		in.Event.OccurredAt = in.Event.OccurredAt.UTC()
		normalized := normalizeEvent(*in.Event)
		in.Event = &normalized
	}
	if in.PendingRevision != nil {
		if in.Event != nil || expected == nil {
			failure(w, invalid("Для очереди нужен действующий план без отдельного события"))
			return
		}
		q, err := s.Store.Pending(r.Context(), r.PathValue("id"))
		if err != nil {
			failure(w, err)
			return
		}
		if q.Revision != *in.PendingRevision || len(q.Events) == 0 || q.SnapshotRevision != in.Revision || q.BasePlanID == nil || *q.BasePlanID != *expected {
			failure(w, c.NewError("STALE_VERSION", "Очередь изменилась или пуста. Обновите смену."))
			return
		}
		event := pendingEvent(q)
		in.Event = &event
	}
	input := storage.ProposalInput{ScenarioID: r.PathValue("id"), RequestID: in.RequestID, SnapshotRevision: in.Revision,
		ExpectedCurrentPlanID: expected, SolveMode: in.SolveMode, Event: in.Event, PendingRevision: in.PendingRevision}
	total := 2
	if input.Event != nil {
		total = 3
	}
	if prior, found, e := s.Store.ProposalByRequest(r.Context(), input.ScenarioID, input.RequestID); e != nil {
		failure(w, e)
		return
	} else if found {
		if !sameProposalInput(prior.ProposalInput, input) {
			failure(w, c.NewError("IDEMPOTENCY_CONFLICT", "Ключ запроса уже использован"))
			return
		}
		respondCalculation(w, r, 200, total, func(context.Context) (any, error) { return prior, nil })
		return
	}
	respondCalculation(w, r, 201, total, func(ctx context.Context) (any, error) {
		var options []c.PlanOption
		var e error
		if input.Event == nil {
			options, e = service.BuildOptions(ctx, c.BuildPlanRequest{SolveMode: input.SolveMode, RequestID: input.RequestID,
				ScenarioID: input.ScenarioID, SnapshotRevision: input.SnapshotRevision, ExpectedCurrentPlanID: expected})
		} else {
			options, e = service.ReplanOptions(ctx, c.ReplanRequest{SolveMode: input.SolveMode, RequestID: input.RequestID,
				ScenarioID: input.ScenarioID, SnapshotRevision: input.SnapshotRevision, BasePlanID: *expected, Event: *input.Event})
		}
		if e != nil {
			return nil, e
		}
		progress.Report(progress.WithState(ctx, progress.State{Completed: total, Total: total}), "saving", "Сохраняем варианты")
		return s.Store.SaveProposal(ctx, input, options)
	})
}

func sameProposalInput(a, b storage.ProposalInput) bool {
	ax, _ := json.Marshal(a)
	bx, _ := json.Marshal(b)
	var x, y any
	_ = json.Unmarshal(ax, &x)
	_ = json.Unmarshal(bx, &y)
	return jsonString(x) == jsonString(y)
}

func jsonString(value any) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func (s *Server) currentProposal(w http.ResponseWriter, r *http.Request) {
	p, e := s.Store.CurrentProposal(r.Context(), r.PathValue("id"))
	respond(w, 200, p, e)
}

func (s *Server) acceptProposal(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RequestID string `json:"request_id"`
		OptionKey string `json:"option_key"`
	}
	if e := decode(r, &in); e != nil {
		failure(w, e)
		return
	}
	plan, e := s.Store.AcceptProposal(r.Context(), r.PathValue("id"), in.RequestID, in.OptionKey)
	respond(w, 200, plan, e)
}
