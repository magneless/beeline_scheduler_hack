//go:build ortools

package httpapi

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

// Exercise real HTTP handlers, PostgreSQL commits, the real optimized solver,
// and the demo geography together. This complements the isolated replay tests.
func TestOptimizedDispatcherDayAudit(t *testing.T) {
	for _, finishOffset := range []time.Duration{-time.Minute, 0, time.Minute} {
		t.Run(fmt.Sprint(finishOffset), func(t *testing.T) {
			h, store, fixture := integrationServerMode(t, c.SolveModeOptimized, true, true)
			snapshot := fixture.Snapshot
			snapshot.ScenarioID = storage.ID("scenario")
			engineer := snapshot.Engineers[0]
			engineer.EquipmentStock = map[c.Equipment]int64{c.EquipmentRouter: 10}
			second, reserve := engineer, engineer
			second.ID, second.Skills, second.SourceOrder = "special-crew", []string{"special"}, 2
			reserve.ID, reserve.Skills, reserve.SourceOrder = "reserve-crew", []string{"reserve"}, 3
			snapshot.Engineers = []c.Engineer{engineer, second, reserve}
			template := snapshot.Orders[0]
			template.WorkType, template.Priority, template.ServiceSec = c.WorkTypeRepair, c.PriorityNormal, 600
			template.Window.End = engineer.Shift.End.Add(-time.Hour)
			snapshot.Orders = nil
			for i := 0; i < 6; i++ {
				o := template
				o.ID = fmt.Sprintf("audit-%d", i)
				o.SourceOrder = int64(i + 1)
				if i >= 3 {
					o.RequiredSkills = []string{"special"}
				}
				if i == 5 {
					o.RequiredSkills = []string{"missing"}
				}
				snapshot.Orders = append(snapshot.Orders, o)
			}
			view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			sid := snapshot.ScenarioID
			var proposal storage.Proposal
			call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{"request_id": "audit-build", "snapshot_revision": view.Snapshot.Revision, "expected_current_plan_id": nil}, 201, &proposal)
			var plan c.Plan
			call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "audit-accept-build", "option_key": "strict"}, 200, &plan)
			call(t, h, "GET", "/scenarios/"+sid, nil, 200, &view)
			if !view.Snapshot.ReserveInitialized || len(plan.Routes) != 2 || len(plan.Unassigned) != 1 {
				t.Fatalf("initial allocation %+v", plan)
			}
			for _, e := range view.Snapshot.Engineers {
				if e.Reserve != (e.ID == reserve.ID) {
					t.Fatal("wrong initial reserve")
				}
			}
			for _, emergency := range []bool{false, true} {
				previous := plan
				o := template
				o.ID = fmt.Sprintf("incoming-%t", emergency)
				o.SourceOrder = 100
				o.ReceivedAt = plan.AsOf.Add(time.Minute)
				kind := "ordinary_order_added"
				if emergency {
					kind = "urgent_order_added"
					o.WorkType, o.Priority, o.ServiceSec = c.WorkTypeEmergency, c.PriorityUrgent, 4800
					o.RequiredSkills = []string{"emergency"}
				}
				call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{"request_id": o.ID, "snapshot_revision": plan.SnapshotRevision, "expected_current_plan_id": plan.ID, "event": map[string]any{"id": o.ID, "type": kind, "occurred_at": o.ReceivedAt, "payload": map[string]any{"order": o}}}, 201, &proposal)
				call(t, h, "GET", "/scenarios/"+sid, nil, 200, &view)
				if view.CurrentPlanID == nil || *view.CurrentPlanID != previous.ID || view.Snapshot.Revision != previous.SnapshotRevision {
					t.Fatal("preview changed accepted plan")
				}
				for _, option := range proposal.Options {
					for _, r := range option.Result.Draft.Routes {
						for _, v := range r.Visits {
							if v.OrderID == "audit-5" {
								t.Fatal("previously unassigned job reentered")
							}
						}
					}
				}
				call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-" + o.ID, "option_key": "strict"}, 200, &plan)
				if !emergency {
					for _, old := range previous.Routes {
						for _, v := range old.Visits {
							found := false
							for _, r := range plan.Routes {
								for _, now := range r.Visits {
									if now.OrderID == v.OrderID {
										found = true
										if r.EngineerID != old.EngineerID || now != v {
											t.Fatal("ordinary insertion moved fixed work")
										}
									}
								}
							}
							if !found {
								t.Fatal("ordinary insertion lost work")
							}
						}
					}
				}
			}
			// Pick the earliest first visit to keep event time monotone.
			route := plan.Routes[0]
			for _, r := range plan.Routes {
				if r.Visits[0].StartAt.Before(route.Visits[0].StartAt) {
					route = r
				}
			}
			visit := route.Visits[0]
			for _, status := range []string{"in_progress", "completed"} {
				at := visit.StartAt
				if status == "completed" {
					at = visit.EndAt.Add(finishOffset)
				}
				previous := plan
				var accepted map[string]string
				call(t, h, "POST", "/plans/"+plan.ID+"/events", map[string]any{"request_id": "audit-" + status, "snapshot_revision": plan.SnapshotRevision, "event": map[string]any{"id": "audit-" + status, "type": "order_status_changed", "occurred_at": at, "payload": map[string]any{"order_id": visit.OrderID, "engineer_id": route.EngineerID, "status": status}}}, 202, &accepted)
				run := awaitRun(t, h, accepted["run_id"])
				if run.Status != "succeeded" {
					t.Fatalf("%s failed: %+v", status, run)
				}
				call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
				if status == "in_progress" && !reflect.DeepEqual(previous.Routes, plan.Routes) {
					t.Fatal("work start changed routes")
				}
				for _, old := range previous.Routes {
					if old.EngineerID == route.EngineerID {
						continue
					}
					found := false
					for _, now := range plan.Routes {
						if now.EngineerID == old.EngineerID {
							found = true
							if !reflect.DeepEqual(old, now) {
								t.Fatal("completion changed another crew")
							}
						}
					}
					if !found {
						t.Fatal("completion removed another crew")
					}
				}
				if status == "completed" && finishOffset <= 0 {
					for _, old := range route.Visits[1:] {
						found := false
						for _, r := range plan.Routes {
							for _, now := range r.Visits {
								if now.OrderID == old.OrderID {
									found = true
									if now != old {
										t.Fatal("timely completion moved future work")
									}
								}
							}
						}
						if !found {
							t.Fatal("timely completion lost future work")
						}
					}
				}
			}
			previous := plan
			var accepted map[string]string
			call(t, h, "POST", "/plans/"+plan.ID+"/events", map[string]any{"request_id": "audit-cancel-deferred", "snapshot_revision": plan.SnapshotRevision, "event": map[string]any{"id": "audit-cancel-deferred", "type": "order_cancelled", "occurred_at": plan.AsOf, "payload": map[string]any{"order_id": "audit-5", "reason": "client_refusal"}}}, 202, &accepted)
			run := awaitRun(t, h, accepted["run_id"])
			if run.Status != "succeeded" {
				t.Fatalf("unassigned cancellation: %+v", run)
			}
			call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
			if !reflect.DeepEqual(previous.Routes, plan.Routes) || !reflect.DeepEqual(previous.EquipmentRemaining, plan.EquipmentRemaining) {
				t.Fatal("unassigned cancellation changed schedule or equipment")
			}
			call(t, h, "GET", "/scenarios/"+sid, nil, 200, &view)
			for _, e := range view.Snapshot.Engineers {
				if e.Reserve != (e.ID == reserve.ID) {
					t.Fatal("events redefined reserve")
				}
			}
		})
	}
}
