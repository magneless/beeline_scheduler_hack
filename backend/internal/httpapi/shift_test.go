package httpapi

import (
	"net/http"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestHTTPFixedShiftImportAndPatch(t *testing.T) {
	h, _, _ := integrationServer(t, true)
	var view c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &view)
	if err := c.ValidateEngineerShifts(view.Snapshot); err != nil {
		t.Fatal(err)
	}
	sid := view.Snapshot.ScenarioID
	path := "/scenarios/" + sid + "/engineers/" + view.Snapshot.Engineers[0].ID
	want := view.Snapshot.Engineers[0].Shift
	for _, bad := range []c.Window{
		{Start: want.Start.Add(-time.Hour), End: want.End},
		{Start: want.Start, End: want.End.Add(time.Hour)},
		{Start: want.Start.Add(24 * time.Hour), End: want.End.Add(24 * time.Hour)},
	} {
		var failure c.ContractError
		call(t, h, "PATCH", path, map[string]any{"expected_revision": 1, "shift": bad}, 422, &failure)
		if failure.Code != "INVALID_INPUT" {
			t.Fatalf("wrong error: %+v", failure)
		}
	}
	for _, hours := range []string{"08:00;23:00", "09:00;22:00", "10:00;23:00", "10:00;21:59"} {
		postRoster(t, h, sid, 1, "valid;repair;car;10:00;22:00;true;0;0\ninvalid;repair;car;"+hours+";true;0;0\n", http.StatusUnprocessableEntity, nil)
	}
	var unchanged c.ScenarioView
	call(t, h, "GET", "/scenarios/"+sid, nil, 200, &unchanged)
	if unchanged.Snapshot.Revision != 1 || unchanged.Snapshot.Engineers[0].ID != view.Snapshot.Engineers[0].ID {
		t.Fatal("invalid input changed roster")
	}
	// Equivalent RFC3339 offsets remain compatible with the PATCH schema.
	zone := time.FixedZone("MSK", 3*60*60)
	call(t, h, "PATCH", path, map[string]any{"expected_revision": 1, "shift": c.Window{Start: want.Start.In(zone), End: want.End.In(zone)}, "available": false}, 200, &view)
	if view.Snapshot.Engineers[0].Available {
		t.Fatal("availability must remain editable")
	}
	if err := c.ValidateEngineerShifts(view.Snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPBuildRespectsMandatoryShift(t *testing.T) {
	h, _, _ := integrationServer(t, true)
	var view c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &view)
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+view.Snapshot.ScenarioID+"/plans", map[string]any{"request_id": "fixed-shift", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("build failed: %+v", run)
	}
	var plan c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
	shift, _ := c.WorkingShift(view.Snapshot.Date, view.Snapshot.Timezone)
	if len(plan.Routes) == 0 {
		t.Fatal("no routes")
	}
	for _, route := range plan.Routes {
		for _, leg := range route.Legs {
			if leg.StartAt.Before(shift.Start) || leg.EndAt.After(shift.End) {
				t.Fatalf("travel outside shift: %+v", leg)
			}
		}
		for _, visit := range route.Visits {
			if visit.StartAt.Before(shift.Start) || visit.EndAt.After(shift.End) {
				t.Fatalf("work outside shift: %+v", visit)
			}
		}
	}
}
