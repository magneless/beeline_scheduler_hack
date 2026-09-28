package httpapi

import (
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"testing"
	"time"
)

func TestIntegratedModulesWithPostgres(t *testing.T) {
	h, _, _ := integrationServer(t, true)
	var v c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &v)
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+v.Snapshot.ScenarioID+"/plans", map[string]any{"request_id": "build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	r := awaitRun(t, h, accepted["run_id"])
	if r.Status != "succeeded" {
		t.Fatalf("build failed: %+v", r)
	}
	var p c.Plan
	call(t, h, "GET", "/plans/"+*r.PlanID, nil, 200, &p)
	if len(p.Routes) != 1 {
		t.Fatal("no computed route")
	}
	if len(p.Routes[0].Legs[0].Geometry) < 2 {
		t.Fatal("missing Go-3 geometry")
	}
	start := p.Routes[0].Visits[0].StartAt
	for n, step := range []struct {
		status string
		at     time.Time
	}{{"in_progress", start}, {"completed", start.Add(time.Duration(v.Snapshot.Orders[0].ServiceSec) * time.Second)}} {
		body := map[string]any{"request_id": step.status, "snapshot_revision": n + 1, "event": map[string]any{"id": step.status, "occurred_at": step.at, "type": "order_status_changed", "payload": map[string]any{"order_id": "order-1", "engineer_id": "eng-1", "status": step.status, "expected_end_at": nil}}}
		call(t, h, "POST", "/plans/"+p.ID+"/events", body, 202, &accepted)
		r = awaitRun(t, h, accepted["run_id"])
		if r.Status != "succeeded" {
			t.Fatalf("%s: %+v", step.status, r.Error)
		}
		call(t, h, "GET", "/plans/"+*r.PlanID, nil, 200, &p)
		if p.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 0 {
			t.Fatal("equipment not consumed exactly once")
		}
	}
	if p.Metrics.CompletedCount != 1 || len(p.CompletedOrderIDs) != 1 {
		t.Fatalf("completion not persisted: %+v", p)
	}
	call(t, h, "GET", "/scenarios/"+v.Snapshot.ScenarioID+"?revision=1", nil, 200, &v)
	if v.Snapshot.Orders[0].Status != c.OrderStatusActive {
		t.Fatal("historical data modified")
	}
}

func TestIntegratedImportAndAllEventKinds(t *testing.T) {
	h, _, _ := integrationServer(t, true)
	var v c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "east"}, 201, &v)
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+v.Snapshot.ScenarioID+"/plans", map[string]any{"request_id": "build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	r := awaitRun(t, h, accepted["run_id"])
	if r.Status != "succeeded" {
		t.Fatal(r.Error)
	}
	var p c.Plan
	call(t, h, "GET", "/plans/"+*r.PlanID, nil, 200, &p)
	if p.Metrics.AssignedCount == 0 {
		t.Fatal("all synthetic orders unassigned")
	}
	order := v.Snapshot.Orders[0]
	order.ID = "incoming"
	order.WorkType = c.WorkTypeEmergency
	order.Priority = c.PriorityUrgent
	order.RequiredSkills = []string{"emergency"}
	order.ServiceSec = 4800
	order.ReceivedAt = p.AsOf
	order.Window.Start = p.AsOf
	order.Window.End = p.AsOf.Add(23 * time.Hour)
	events := []struct {
		kind    string
		payload any
	}{{"urgent_order_added", map[string]any{"order": order}}, {"order_cancelled", map[string]any{"order_id": "incoming", "reason": "client_refusal"}}, {"engineer_unavailable", map[string]any{"engineer_id": v.Snapshot.Engineers[0].ID}}}
	for n, e := range events {
		call(t, h, "POST", "/plans/"+p.ID+"/events", map[string]any{"request_id": e.kind, "snapshot_revision": n + 1, "event": map[string]any{"id": e.kind, "occurred_at": p.AsOf, "type": e.kind, "payload": e.payload}}, 202, &accepted)
		r = awaitRun(t, h, accepted["run_id"])
		if r.Status != "succeeded" {
			t.Fatalf("%s: %+v", e.kind, r.Error)
		}
		call(t, h, "GET", "/plans/"+*r.PlanID, nil, 200, &p)
	}
}
