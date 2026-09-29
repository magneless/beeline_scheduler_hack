package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

type pendingService interface {
	PreparePending(context.Context, c.ReplanRequest) (c.Snapshot, c.Event, error)
}

func pendingEvent(q storage.PendingChanges) c.Event {
	latest := q.Events[0].OccurredAt
	for _, event := range q.Events {
		if event.OccurredAt.After(latest) {
			latest = event.OccurredAt
		}
	}
	body, _ := json.Marshal(map[string]any{"events": q.Events})
	return c.Event{ID: fmt.Sprintf("pending-%s-%d", q.ScenarioID, q.Revision), Type: "pending_changes", OccurredAt: latest, Payload: body}
}

func (s *Server) getPending(w http.ResponseWriter, r *http.Request) {
	q, err := s.Store.Pending(r.Context(), r.PathValue("id"))
	respond(w, 200, q, err)
}

func (s *Server) savePending(w http.ResponseWriter, r *http.Request) {
	service, ok := s.Plans.(pendingService)
	if !ok {
		failure(w, invalid("Сервис событий недоступен"))
		return
	}
	var in struct {
		Revision         int64    `json:"expected_revision"`
		SnapshotRevision int64    `json:"snapshot_revision"`
		BasePlanID       string   `json:"base_plan_id"`
		Event            *c.Event `json:"event"`
		UndoLast         bool     `json:"undo_last"`
	}
	if err := decode(r, &in); err != nil {
		failure(w, err)
		return
	}
	if (in.Event == nil) == !in.UndoLast {
		failure(w, invalid("Укажите событие или отмену последнего изменения"))
		return
	}
	q, err := s.Store.Pending(r.Context(), r.PathValue("id"))
	if err != nil {
		failure(w, err)
		return
	}
	if q.Revision != in.Revision || q.SnapshotRevision != in.SnapshotRevision || q.BasePlanID == nil || *q.BasePlanID != in.BasePlanID {
		failure(w, c.NewError("STALE_VERSION", "План или очередь уже изменились. Обновите смену."))
		return
	}
	if in.UndoLast {
		if len(q.Events) == 0 {
			failure(w, invalid("Очередь пуста"))
			return
		}
		q.Events = q.Events[:len(q.Events)-1]
	} else {
		if err = validateEvent(*in.Event); err != nil {
			failure(w, err)
			return
		}
		ev := normalizeEvent(*in.Event)
		ev.OccurredAt = ev.OccurredAt.UTC()
		q.Events = append(q.Events, ev)
	}
	q.Snapshot = nil
	if len(q.Events) > 0 {
		target, normalized, e := service.PreparePending(r.Context(), c.ReplanRequest{ScenarioID: q.ScenarioID, SnapshotRevision: q.SnapshotRevision, BasePlanID: *q.BasePlanID, Event: pendingEvent(q)})
		if e != nil {
			failure(w, e)
			return
		}
		var payload struct {
			Events []c.Event `json:"events"`
		}
		if e = json.Unmarshal(normalized.Payload, &payload); e != nil {
			failure(w, e)
			return
		}
		q.Events, q.Snapshot = payload.Events, &target
	}
	q, err = s.Store.SavePending(r.Context(), q, in.Revision)
	respond(w, 200, q, err)
}
