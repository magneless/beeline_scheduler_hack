//go:build vroom

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

// These HTTP/storage/planning-service assertions use the production solver.
// No mocked or local experimental solver participates in this test.
func TestSolverDecisionDispatcherConditions(t *testing.T) {
	for _, phase := range []string{"trip", "work"} {
		for _, eventType := range []string{"ordinary_order_added", "urgent_order_added", "order_cancelled", "engineer_unavailable"} {
			t.Run(phase+"/"+eventType, func(t *testing.T) {
				h, store, f := integrationServerMode(t, c.SolveModeOptimized, true, true)
				s := f.Snapshot
				s.ScenarioID = storage.ID("decision")
				s.Engineers[0].EquipmentStock = map[c.Equipment]int64{c.EquipmentRouter: 10}
				s.Orders = nil
				for i := 0; i < 3; i++ {
					o := f.Snapshot.Orders[0]
					o.ID = fmt.Sprintf("visit-%d", i)
					o.SourceOrder = int64(i)
					o.Window.End = s.Engineers[0].Shift.End.Add(-time.Hour)
					s.Orders = append(s.Orders, o)
				}
				view, err := store.CreateScenario(context.Background(), s, map[string]any{})
				if err != nil {
					t.Fatal(err)
				}
				var proposal storage.Proposal
				call(t, h, "POST", "/scenarios/"+s.ScenarioID+"/proposals", map[string]any{"request_id": "initial", "snapshot_revision": view.Snapshot.Revision, "expected_current_plan_id": nil}, 201, &proposal)
				var plan c.Plan
				call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-initial", "option_key": "strict"}, 200, &plan)
				if len(plan.Routes) != 1 || len(plan.Routes[0].Visits) != 3 {
					t.Fatalf("initial tasks lost: %+v", plan)
				}
				first := plan.Routes[0].Visits[0]
				firstLeg := plan.Routes[0].Legs[0]
				at := firstLeg.StartAt.Add(firstLeg.EndAt.Sub(firstLeg.StartAt) / 2).Truncate(time.Second)
				if phase == "work" {
					var accepted map[string]string
					call(t, h, "POST", "/plans/"+plan.ID+"/events", map[string]any{"request_id": "start", "snapshot_revision": plan.SnapshotRevision, "event": map[string]any{"id": "start", "type": "order_status_changed", "occurred_at": first.StartAt, "payload": map[string]any{"order_id": first.OrderID, "engineer_id": plan.Routes[0].EngineerID, "status": "in_progress", "expected_end_at": first.EndAt}}}, 202, &accepted)
					run := awaitRun(t, h, accepted["run_id"])
					if run.Status != "succeeded" {
						t.Fatal(run.Error)
					}
					call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
					at = first.StartAt.Add(time.Minute)
				}
				payload := map[string]any{}
				switch eventType {
				case "ordinary_order_added", "urgent_order_added":
					o := s.Orders[0]
					o.ID = "incoming"
					o.SourceOrder = 99
					o.ReceivedAt = at
					o.Window.Start = at
					if eventType == "urgent_order_added" {
						o.WorkType = c.WorkTypeEmergency
						o.Priority = c.PriorityUrgent
						o.ServiceSec = 4800
					}
					payload["order"] = o
				case "order_cancelled":
					payload["order_id"] = plan.Routes[0].Visits[2].OrderID
					payload["reason"] = "client_refusal"
				case "engineer_unavailable":
					payload["engineer_id"] = plan.Routes[0].EngineerID
				}
				before := plan
				call(t, h, "POST", "/scenarios/"+s.ScenarioID+"/proposals", map[string]any{"request_id": "event", "snapshot_revision": plan.SnapshotRevision, "expected_current_plan_id": plan.ID, "event": map[string]any{"id": "event", "type": eventType, "occurred_at": at, "payload": payload}}, 201, &proposal)
				keys := map[string]bool{}
				for _, option := range proposal.Options {
					keys[option.Key] = true
				}
				if !keys["strict"] || !keys["reserve"] {
					t.Fatalf("missing current/reserve choices: %v", keys)
				}
				if eventType == "urgent_order_added" {
					if !keys["late_emergency"] || keys["original"] {
						t.Fatalf("incorrect emergency choices: %v", keys)
					}
				} else if !keys["original"] {
					t.Fatal("original plan choice is missing")
				}
				call(t, h, "GET", "/scenarios/"+s.ScenarioID, nil, 200, &view)
				if view.CurrentPlanID == nil || *view.CurrentPlanID != before.ID || view.Snapshot.Revision != before.SnapshotRevision {
					t.Fatal("preview mutated accepted day")
				}
				call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-event", "option_key": "strict"}, 200, &plan)
				found := false
				for _, r := range plan.Routes {
					for _, v := range r.Visits {
						if v.OrderID == first.OrderID {
							found = true
							if phase == "work" || eventType != "urgent_order_added" {
								if v != first || r.EngineerID != before.Routes[0].EngineerID {
									t.Fatalf("protected work/trip appointment moved: before=%+v after=%+v crew=%s", first, v, r.EngineerID)
								}
							}
						}
					}
				}
				if (phase == "work" || eventType != "urgent_order_added") && !found {
					t.Fatal("protected appointment lost")
				}
				if eventType == "ordinary_order_added" {
					for _, v := range before.Routes[0].Visits {
						present := false
						for _, r := range plan.Routes {
							for _, w := range r.Visits {
								if w.OrderID == v.OrderID {
									present = true
									if w != v {
										t.Fatal("ordinary insertion shifted visit")
									}
								}
							}
						}
						if !present {
							t.Fatal("ordinary insertion removed visit")
						}
					}
				}
				if eventType != "order_cancelled" && !reflect.DeepEqual(before.CancelledOrderIDs, plan.CancelledOrderIDs) {
					t.Fatal("event automatically cancelled an unrelated task")
				}
			})
		}
	}
}

func TestSolverDecisionLateAndReserveOptions(t *testing.T) {
	for _, variant := range []string{"late", "reserve"} {
		t.Run(variant, func(t *testing.T) {
			h, store, f := integrationServerMode(t, c.SolveModeOptimized, true, true)
			s := f.Snapshot
			s.ScenarioID = storage.ID("decision")
			if variant == "late" {
				s.Orders[0].WorkType = c.WorkTypeEmergency
				s.Orders[0].Priority = c.PriorityUrgent
				s.Orders[0].ServiceSec = 4800
				s.Orders[0].Window = c.Window{Start: s.Engineers[0].Shift.Start, End: s.Engineers[0].Shift.Start}
			} else {
				r := s.Engineers[0]
				r.ID = "reserve"
				r.SourceOrder = 2
				r.Skills = []string{"reserve"}
				s.Engineers = append(s.Engineers, r)
			}
			view, err := store.CreateScenario(context.Background(), s, map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			var proposal storage.Proposal
			call(t, h, "POST", "/scenarios/"+s.ScenarioID+"/proposals", map[string]any{"request_id": "initial", "snapshot_revision": view.Snapshot.Revision, "expected_current_plan_id": nil}, 201, &proposal)
			var plan c.Plan
			if variant == "late" {
				for _, o := range proposal.Options {
					if o.Key == "strict" && o.Result.Draft.Metrics.AssignedCount != 0 {
						t.Fatal("hard emergency window was relaxed")
					}
					if o.Key == "late_emergency" {
						if o.Result.Draft.Metrics.AssignedCount != 1 || len(o.Result.Draft.Lateness) != 1 || o.Result.Draft.Lateness[0].LateSec <= 0 {
							t.Fatalf("late emergency not explicitly represented: %+v", o)
						}
					}
				}
				call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept", "option_key": "late_emergency"}, 200, &plan)
				call(t, h, "GET", "/scenarios/"+s.ScenarioID, nil, 200, &view)
				if view.Snapshot.Orders[0].Window != s.Orders[0].Window {
					t.Fatal("original emergency window overwritten")
				}
				return
			}
			call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept", "option_key": "strict"}, 200, &plan)
			o := s.Orders[0]
			o.ID = "needs-reserve"
			o.RequiredSkills = []string{"reserve"}
			o.ReceivedAt = plan.AsOf
			call(t, h, "POST", "/scenarios/"+s.ScenarioID+"/proposals", map[string]any{"request_id": "reserve-event", "snapshot_revision": plan.SnapshotRevision, "expected_current_plan_id": plan.ID, "event": map[string]any{"id": "reserve-event", "type": "ordinary_order_added", "occurred_at": plan.AsOf, "payload": map[string]any{"order": o}}}, 201, &proposal)
			for _, option := range proposal.Options {
				assigned := ""
				for _, r := range option.Result.Draft.Routes {
					for _, v := range r.Visits {
						if v.OrderID == o.ID {
							assigned = r.EngineerID
						}
					}
				}
				if option.Key == "strict" && assigned != "" {
					t.Fatal("reserve used without selecting reserve option")
				}
				if option.Key == "reserve" && assigned != "reserve" {
					t.Fatalf("original reserve not activated: %+v", option)
				}
			}
			call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-reserve", "option_key": "reserve"}, 200, &plan)
			call(t, h, "GET", "/scenarios/"+s.ScenarioID, nil, 200, &view)
			if !view.Snapshot.ReserveInitialized {
				t.Fatal("reserve definition lost")
			}
			for _, e := range view.Snapshot.Engineers {
				if e.ID == "reserve" && e.Reserve {
					t.Fatal("accepted reserve still in reserve")
				}
			}
		})
	}
}
