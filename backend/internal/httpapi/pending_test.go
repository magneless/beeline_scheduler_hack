package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/geo"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/plans"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

func TestPendingUndoAndConcurrentSaves(t *testing.T) {
	h, _, _, counter, snapshot, plan := pendingDay(t)
	path := "/scenarios/" + plan.ScenarioID + "/pending-changes"
	results := make(chan int, 2)
	for _, id := range []string{"a", "b"} {
		order := snapshot.Orders[0]
		order.ID, order.ReceivedAt = id, plan.AsOf
		ev := eventBody(id, "ordinary_order_added", plan.AsOf, map[string]any{"order": order})
		body, _ := json.Marshal(map[string]any{"expected_revision": 0, "snapshot_revision": plan.SnapshotRevision, "base_plan_id": plan.ID, "event": ev})
		go func(body []byte) {
			r := httptest.NewRequest("POST", "/api/v1"+path, bytes.NewReader(body))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			results <- w.Code
		}(body)
	}
	a, b := <-results, <-results
	if !(a == 200 && b == 409 || b == 200 && a == 409) {
		t.Fatalf("concurrent saves: %d %d", a, b)
	}
	var q storage.PendingChanges
	call(t, h, "GET", path, nil, 200, &q)
	if len(q.Events) != 1 || counter.calls != 0 {
		t.Fatal("concurrent queue lost changes or calculated routes")
	}
	revision := q.Revision
	q = storage.PendingChanges{}
	call(t, h, "POST", path, map[string]any{"expected_revision": revision, "snapshot_revision": plan.SnapshotRevision, "base_plan_id": plan.ID, "undo_last": true}, 200, &q)
	if len(q.Events) != 0 || q.Snapshot != nil || q.Revision != 2 {
		t.Fatal("undo did not restore empty queue")
	}
	call(t, h, "POST", path, map[string]any{"expected_revision": q.Revision, "snapshot_revision": plan.SnapshotRevision, "base_plan_id": plan.ID, "undo_last": true}, 422, nil)
}

type pendingCountingPlanner struct {
	calls    int
	delegate *planner.Planner
}

func (p *pendingCountingPlanner) Solve(ctx context.Context, in c.SolveRequest) (c.SolveResult, error) {
	p.calls++
	return p.delegate.Solve(ctx, in)
}

func pendingDay(t *testing.T) (http.Handler, *storage.Store, *plans.Service, *pendingCountingPlanner, c.Snapshot, c.Plan) {
	t.Helper()
	_, store, f := integrationServer(t)
	snapshot := f.Snapshot
	snapshot.ScenarioID = storage.ID("pending")
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	counter := &pendingCountingPlanner{delegate: planner.New()}
	svc, err := plans.New(store, geo.NewGeoService(&geo.DemoProvider{}), counter, plans.Options{Mode: c.SolveModeBaseline})
	if err != nil {
		t.Fatal(err)
	}
	h := (&Server{Store: store, Plans: svc}).Handler()
	var proposal storage.Proposal
	call(t, h, "POST", "/scenarios/"+snapshot.ScenarioID+"/proposals", map[string]any{"request_id": "initial", "snapshot_revision": view.Snapshot.Revision, "expected_current_plan_id": nil}, 201, &proposal)
	var plan c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-initial", "option_key": "strict"}, 200, &plan)
	counter.calls = 0
	return h, store, svc, counter, snapshot, plan
}

func queueEvent(t *testing.T, h http.Handler, plan c.Plan, revision int64, ev c.Event, status int) storage.PendingChanges {
	t.Helper()
	var queue storage.PendingChanges
	call(t, h, "POST", "/scenarios/"+plan.ScenarioID+"/pending-changes", map[string]any{"expected_revision": revision, "snapshot_revision": plan.SnapshotRevision, "base_plan_id": plan.ID, "event": ev}, status, &queue)
	return queue
}

func eventBody(id, kind string, at time.Time, payload any) c.Event {
	body, _ := json.Marshal(payload)
	return c.Event{ID: id, Type: kind, OccurredAt: at, Payload: body}
}

func TestPendingChangesRequireManualCalculationAndFreshAcceptance(t *testing.T) {
	h, store, svc, counter, snapshot, plan := pendingDay(t)
	add := func(id string) c.Event {
		order := snapshot.Orders[0]
		order.ID, order.ReceivedAt = id, plan.AsOf
		return eventBody("event-"+id, "ordinary_order_added", plan.AsOf, map[string]any{"order": order})
	}
	q := queueEvent(t, h, plan, 0, add("new-a"), 200)
	q = queueEvent(t, h, plan, q.Revision, add("new-b"), 200)
	if counter.calls != 0 || len(q.Events) != 2 || len(q.Snapshot.Orders) != len(snapshot.Orders)+2 {
		t.Fatalf("queue computed routes or lost changes: calls=%d queue=%+v", counter.calls, q)
	}
	reopened := (&Server{Store: store, Plans: svc}).Handler()
	var restored storage.PendingChanges
	call(t, reopened, "GET", "/scenarios/"+plan.ScenarioID+"/pending-changes", nil, 200, &restored)
	a, _ := json.Marshal(restored)
	b, _ := json.Marshal(q)
	var x, y any
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	if !reflect.DeepEqual(x, y) {
		t.Fatal("pending changes were not persisted")
	}
	view, err := store.GetScenario(context.Background(), plan.ScenarioID, 0)
	if err != nil || *view.CurrentPlanID != plan.ID || len(view.Snapshot.Orders) != len(snapshot.Orders) {
		t.Fatal("saving queue mutated accepted plan", err)
	}
	queueEvent(t, h, plan, 0, add("stale"), 409)
	var proposal storage.Proposal
	calculate := func(id string) {
		call(t, h, "POST", "/scenarios/"+plan.ScenarioID+"/proposals", map[string]any{"request_id": id, "snapshot_revision": plan.SnapshotRevision, "expected_current_plan_id": plan.ID, "pending_revision": q.Revision}, 201, &proposal)
	}
	calculate("manual-first")
	if counter.calls == 0 {
		t.Fatal("manual calculation did not call planner")
	}
	for _, before := range plan.Routes[0].Visits {
		found := false
		for _, route := range proposal.Options[0].Result.Draft.Routes {
			for _, visit := range route.Visits {
				if before.OrderID == visit.OrderID {
					found = true
					if before != visit {
						t.Fatal("ordinary additions moved accepted visit")
					}
				}
			}
		}
		if !found {
			t.Fatal("ordinary additions removed accepted visit")
		}
	}
	counter.calls = 0
	q = queueEvent(t, h, plan, q.Revision, eventBody("cancel", "order_cancelled", plan.AsOf, map[string]any{"order_id": snapshot.Orders[0].ID, "reason": "client_refusal"}), 200)
	if counter.calls != 0 {
		t.Fatal("cancellation triggered calculation")
	}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "stale-accept", "option_key": "strict"}, 409, nil)
	var current *storage.Proposal
	call(t, h, "GET", "/scenarios/"+plan.ScenarioID+"/proposals/current", nil, 200, &current)
	if current != nil {
		t.Fatal("stale proposal remains visible")
	}
	calculate("manual-second")
	var accepted c.Plan
	accept := map[string]any{"request_id": "accept-queue", "option_key": "strict"}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", accept, 200, &accepted)
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", accept, 200, nil)
	call(t, h, "GET", "/scenarios/"+plan.ScenarioID+"/pending-changes", nil, 200, &restored)
	if len(restored.Events) != 0 || restored.Revision <= q.Revision {
		t.Fatal("acceptance did not clear queue")
	}
	if accepted.SnapshotRevision != plan.SnapshotRevision+1 {
		t.Fatal("queue must commit one snapshot")
	}
	if !reflect.DeepEqual(accepted.CancelledOrderIDs, []string{snapshot.Orders[0].ID}) {
		t.Fatal("cancellation lost")
	}
}

func TestPendingUnavailableRemovesTasksPermanentlyFromAutomaticPlanning(t *testing.T) {
	h, _, _, counter, snapshot, plan := pendingDay(t)
	q := queueEvent(t, h, plan, 0, eventBody("disabled", "engineer_unavailable", plan.AsOf, map[string]any{"engineer_id": plan.Routes[0].EngineerID}), 200)
	if counter.calls != 0 {
		t.Fatal("unavailability calculated automatically")
	}
	var proposal storage.Proposal
	calculate := func(id string) {
		call(t, h, "POST", "/scenarios/"+plan.ScenarioID+"/proposals", map[string]any{"request_id": id, "snapshot_revision": plan.SnapshotRevision, "expected_current_plan_id": plan.ID, "pending_revision": q.Revision}, 201, &proposal)
	}
	calculate("remove")
	for _, option := range proposal.Options {
		if option.Key == "original" {
			t.Fatal("unavailable engineer has original option")
		}
	}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "invalid-original", "option_key": "original"}, 422, nil)
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "remove-accept", "option_key": "remove_unavailable"}, 200, &plan)
	if len(plan.Unassigned) != len(snapshot.Orders) || len(plan.DeferredOrderIDs) != len(snapshot.Orders) {
		t.Fatal("remaining work not moved to deferred list")
	}
	call(t, h, "GET", "/scenarios/"+plan.ScenarioID+"/pending-changes", nil, 200, &q)
	order := snapshot.Orders[0]
	order.ID, order.WorkType, order.Priority, order.ServiceSec, order.ReceivedAt = "emergency", c.WorkTypeEmergency, c.PriorityUrgent, 4800, plan.AsOf
	q = queueEvent(t, h, plan, q.Revision, eventBody("emergency-added", "urgent_order_added", plan.AsOf, map[string]any{"order": order}), 200)
	calculate("emergency-options")
	for _, option := range proposal.Options {
		for _, route := range option.Result.Draft.Routes {
			for _, visit := range route.Visits {
				if visit.OrderID == snapshot.Orders[0].ID {
					t.Fatal("deferred work returned after emergency")
				}
			}
		}
	}
}
