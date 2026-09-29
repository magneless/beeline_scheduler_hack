package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

func workFactDay(t *testing.T) (http.Handler, *storage.Store, c.ScenarioView, c.Plan) {
	t.Helper()
	h, store, fixture := integrationServer(t, true, true)
	snapshot := fixture.Snapshot
	snapshot.ScenarioID = storage.ID("work-facts")
	snapshot.Engineers[0].EquipmentStock[c.EquipmentRouter] = 4
	next := snapshot.Orders[0]
	next.ID, next.SourceOrder = "order-2", 2
	next.Window.Start = next.Window.Start.Add(time.Hour)
	snapshot.Orders = append(snapshot.Orders, next)
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var proposal storage.Proposal
	call(t, h, "POST", "/scenarios/"+snapshot.ScenarioID+"/proposals", map[string]any{"request_id": "initial", "snapshot_revision": view.Snapshot.Revision, "expected_current_plan_id": nil}, 201, &proposal)
	var plan c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-initial", "option_key": "strict"}, 200, &plan)
	if len(plan.Routes) != 1 || len(plan.Routes[0].Visits) != 2 {
		t.Fatal("fixture must assign both visits to one crew")
	}
	return h, store, view, plan
}

func TestWorkFactsPersistInReverseOrder(t *testing.T) {
	for _, mode := range []string{"direct", "queued", "historical_queue", "historical_late_queue"} {
		t.Run(mode, func(t *testing.T) {
			queue := mode != "direct"
			h, store, view, plan := workFactDay(t)
			initial := plan
			first, second := plan.Routes[0].Visits[0], plan.Routes[0].Visits[1]
			if mode == "historical_late_queue" {
				first.EndAt = first.EndAt.Add(5 * time.Minute)
			}
			events := []c.Event{
				eventBody("second-start", "order_status_changed", second.StartAt, map[string]any{"order_id": second.OrderID, "engineer_id": "eng-1", "status": "in_progress"}),
				eventBody("second-finish", "order_status_changed", second.EndAt, map[string]any{"order_id": second.OrderID, "engineer_id": "eng-1", "status": "completed"}),
				eventBody("first-start", "order_status_changed", first.StartAt, map[string]any{"order_id": first.OrderID, "engineer_id": "eng-1", "status": "in_progress"}),
				eventBody("first-finish", "order_status_changed", first.EndAt, map[string]any{"order_id": first.OrderID, "engineer_id": "eng-1", "status": "completed"}),
			}
			var pending storage.PendingChanges
			for index, event := range events {
				if queue && (!strings.HasPrefix(mode, "historical") || index >= 2) {
					pending = queueEvent(t, h, plan, pending.Revision, event, 200)
				} else {
					path := "/plans/" + plan.ID + "/events"
					body := map[string]any{"request_id": event.ID, "snapshot_revision": plan.SnapshotRevision, "event": event}
					var accepted map[string]string
					call(t, h, "POST", path, body, 202, &accepted)
					run := awaitRun(t, h, accepted["run_id"])
					if run.Status != "succeeded" {
						t.Fatalf("fact failed: %+v", run.Error)
					}
					var repeated map[string]string
					call(t, h, "POST", path, body, 202, &repeated)
					if repeated["run_id"] != accepted["run_id"] {
						t.Fatal("fact was applied twice")
					}
					call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
				}
			}
			if queue {
				expectedEvents := events
				if strings.HasPrefix(mode, "historical") {
					expectedEvents = events[2:]
				}
				call(t, h, "POST", "/scenarios/"+plan.ScenarioID+"/pending-changes", map[string]any{"expected_revision": pending.Revision, "snapshot_revision": plan.SnapshotRevision, "base_plan_id": plan.ID, "undo_last": true}, 200, &pending)
				if len(pending.Events) != len(expectedEvents)-1 || pending.Events[len(pending.Events)-1].ID != "first-start" || pending.Snapshot.Orders[0].Status != c.OrderStatusInProgress {
					t.Fatal("undo removed chronological last instead of last entered")
				}
				pending = queueEvent(t, h, plan, pending.Revision, events[3], 200)
				// Same backing store, fresh handler: entry order and exact times survive reload.
				reader := (&Server{Store: store}).Handler()
				call(t, reader, "GET", "/scenarios/"+plan.ScenarioID+"/pending-changes", nil, 200, &pending)
				if len(pending.Events) != len(expectedEvents) {
					t.Fatal("pending count changed")
				}
				for i, actual := range pending.Events {
					expected := expectedEvents[i]
					var actualPayload, expectedPayload c.OrderStatusChanged
					if err := json.Unmarshal(actual.Payload, &actualPayload); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(expected.Payload, &expectedPayload); err != nil {
						t.Fatal(err)
					}
					if actual.ID != expected.ID || !actual.OccurredAt.Equal(expected.OccurredAt) || !reflect.DeepEqual(actualPayload, expectedPayload) {
						t.Fatal("persisted fact or entry order changed")
					}
				}
				var proposal storage.Proposal
				call(t, h, "POST", "/scenarios/"+plan.ScenarioID+"/proposals", map[string]any{"request_id": "calculate", "snapshot_revision": plan.SnapshotRevision, "expected_current_plan_id": plan.ID, "pending_revision": pending.Revision}, 201, &proposal)
				call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-facts", "option_key": "strict"}, 200, &plan)
			}
			call(t, (&Server{Store: store}).Handler(), "GET", "/scenarios/"+plan.ScenarioID, nil, 200, &view)
			for _, order := range view.Snapshot.Orders {
				if order.Status != c.OrderStatusCompleted {
					t.Fatal("completion was lost")
				}
			}
			if mode == "historical_late_queue" {
				initial.Routes[0].Visits[0].EndAt = first.EndAt
			}
			if !plan.AsOf.Equal(second.EndAt) || !reflect.DeepEqual(plan.Routes, initial.Routes) || plan.Metrics.CompletedCount != 2 || plan.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 2 {
				t.Fatal("clock, route, count or stock depends on reporting order")
			}
		})
	}
}
