package httpapi

import (
	"context"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"testing"
)

func TestResolveImportedAddressPreservesWorkAndHistory(t *testing.T) {
	h, store, fixture := integrationServer(t, true)
	snapshot := data.Clone(fixture.Snapshot)
	snapshot.ScenarioID = ""
	order := snapshot.Orders[0]
	order.ID = "unlocated"
	order.LocationID = "unlocated-location"
	order.SourceOrder = 2
	snapshot.UnlocatedOrders = []c.UnlocatedOrder{{Order: order, Address: "Москва, неизвестный формат", Message: "Нужна точка"}}
	id := order.LocationID
	snapshot.Issues = append(snapshot.Issues, c.Issue{EntityID: &id, Code: "GEO_UNAVAILABLE", Message: "Нужна точка"})
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	path := "/scenarios/" + view.Snapshot.ScenarioID + "/orders/" + order.ID + "/address"
	point := c.Point{Lat: 55.762, Lon: 37.615}
	call(t, h, "POST", path, map[string]any{"snapshot_revision": 1, "address": "Москва, Петровка, 2", "point": map[string]any{"lat": 95, "lon": 37}}, 422, nil)
	call(t, h, "POST", path, map[string]any{"snapshot_revision": 1, "address": "Москва, Петровка, 2"}, 422, nil)
	var updated c.ScenarioView
	call(t, h, "POST", path, map[string]any{"snapshot_revision": 1, "address": "Москва, Петровка, 2", "point": point}, 200, &updated)
	if updated.Snapshot.Revision != 2 || len(updated.Snapshot.UnlocatedOrders) != 0 || len(updated.Snapshot.Orders) != 2 {
		t.Fatalf("%+v", updated)
	}
	restored := updated.Snapshot.Orders[1]
	if restored.ID != order.ID || restored.WorkType != order.WorkType || restored.Priority != order.Priority || restored.Window != order.Window || restored.SourceOrder != order.SourceOrder {
		t.Fatalf("work altered: %+v", restored)
	}
	found := false
	for _, loc := range updated.Snapshot.Locations {
		if loc.ID == order.LocationID {
			found = loc.Point == point
		}
	}
	if !found {
		t.Fatal("confirmed coordinates were not saved")
	}
	for _, issue := range updated.Snapshot.Issues {
		if issue.EntityID != nil && *issue.EntityID == order.LocationID {
			t.Fatal("resolved address still marked invalid")
		}
	}
	old, err := store.GetSnapshot(context.Background(), view.Snapshot.ScenarioID, 1)
	if err != nil || len(old.UnlocatedOrders) != 1 || len(old.Orders) != 1 {
		t.Fatalf("history modified: %+v %v", old, err)
	}
	call(t, h, "POST", path, map[string]any{"snapshot_revision": 1, "address": "Москва, Петровка, 2", "point": point}, 409, nil)
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+view.Snapshot.ScenarioID+"/plans", map[string]any{"request_id": "build-restored", "snapshot_revision": 2, "expected_current_plan_id": nil}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatal(run.Error)
	}
	var plan c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
	accounted := false
	for _, route := range plan.Routes {
		for _, visit := range route.Visits {
			if visit.OrderID == order.ID {
				accounted = true
			}
		}
	}
	for _, item := range plan.Unassigned {
		if item.OrderID == order.ID {
			accounted = true
		}
	}
	if !accounted {
		t.Fatal("restored request never reached routing")
	}
	call(t, h, "POST", path, map[string]any{"snapshot_revision": plan.SnapshotRevision, "address": "Москва, Петровка, 2", "point": point}, 409, nil)
}
