package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

func TestHTTPSavedScenariosUseCurrentSnapshotsWithoutGeocoding(t *testing.T) {
	h, store, fixture := integrationServer(t)
	// This handler has no importer or geocoder: listing saved work needs neither.
	readOnly := (&Server{Store: store}).Handler()
	var list struct {
		Items []c.ScenarioSummary `json:"items"`
	}
	call(t, readOnly, "GET", "/scenarios", nil, 200, &list)
	if list.Items == nil || len(list.Items) != 0 {
		t.Fatalf("empty list must be [], got %#v", list.Items)
	}
	ctx := context.Background()
	first := data.Clone(fixture.Snapshot)
	first.ScenarioID = "saved-first"
	first.RegionID = "east"
	created, err := store.CreateScenario(ctx, first, nil)
	if err != nil {
		t.Fatal(err)
	}
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+first.ScenarioID+"/plans", map[string]any{
		"request_id": "build-saved", "snapshot_revision": 1, "expected_current_plan_id": nil,
	}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("build: %+v", run)
	}

	second := data.Clone(first)
	second.ScenarioID = "saved-second"
	missing := data.Clone(second.Orders[0])
	missing.ID, missing.LocationID = "unlocated", "unlocated-location"
	second.UnlocatedOrders = []c.UnlocatedOrder{{Order: missing, Address: "Требует уточнения"}}
	if _, err := store.CreateScenario(ctx, second, nil); err != nil {
		t.Fatal(err)
	}

	// Legacy imports can retain an unresolved address only in import metadata.
	legacy := data.Clone(first)
	legacy.ScenarioID, legacy.RegionID = "saved-manual", "custom"
	day, _ := time.Parse("2006-01-02", legacy.Date)
	legacy.Date = day.AddDate(0, 0, 1).Format("2006-01-02")
	shift, err := c.WorkingShift(legacy.Date, legacy.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	for i := range legacy.Engineers {
		legacy.Engineers[i].Shift = shift
	}
	lid := "custom-location-legacy"
	legacy.Issues = append(legacy.Issues, c.Issue{EntityID: &lid, Code: "GEOCODE_NOT_FOUND", Message: "Уточните адрес"})
	date := day.AddDate(0, 0, 1).Format("02.01.2006")
	metadata := data.ImportMetadata{SourceRows: [][]string{{
		"legacy", "Подключение", "Конвергенция абонента", date + " 10:00", date + " 12:00", "", "Москва, неизвестный адрес",
	}}}
	if _, err := store.CreateScenario(ctx, legacy, metadata); err != nil {
		t.Fatal(err)
	}

	call(t, readOnly, "GET", "/scenarios", nil, 200, &list)
	if len(list.Items) != 3 || list.Items[0].ScenarioID != legacy.ScenarioID {
		t.Fatalf("missing saved copies or wrong date order: %+v", list.Items)
	}
	for _, item := range list.Items {
		var view c.ScenarioView
		call(t, readOnly, "GET", "/scenarios/"+item.ScenarioID, nil, 200, &view)
		if item.Revision != view.Snapshot.Revision || item.OrderCount != len(view.Snapshot.Orders)+len(view.Snapshot.UnlocatedOrders) || item.UnlocatedCount != len(view.Snapshot.UnlocatedOrders) || !reflect.DeepEqual(item.CurrentPlanID, view.CurrentPlanID) {
			t.Fatalf("summary differs from saved day: %+v", item)
		}
	}
	if list.Items[0].UnlocatedCount != 1 || list.Items[1].CurrentPlanID == nil || *list.Items[1].CurrentPlanID != *run.PlanID || list.Items[2].UnlocatedCount != 1 {
		t.Fatalf("unresolved addresses or accepted plan lost: %+v", list.Items)
	}

	point := c.Point{Lat: 55.7173565, Lon: 37.7982718}
	resolved, err := store.ResolveOrderAddress(ctx, second.ScenarioID, missing.ID, 1, c.Location{Address: "Уточнённый адрес", Point: point})
	if err != nil {
		t.Fatal(err)
	}
	call(t, readOnly, "GET", "/scenarios", nil, 200, &list)
	if len(list.Items) != 3 || list.Items[2].Revision != 2 || list.Items[2].UnlocatedCount != 0 || list.Items[2].OrderCount != len(second.Orders)+1 {
		t.Fatalf("list duplicated history or used an old revision: %+v", list.Items)
	}
	var reopened c.ScenarioView
	call(t, readOnly, "GET", "/scenarios/"+second.ScenarioID, nil, 200, &reopened)
	want, _ := json.Marshal(resolved)
	got, _ := json.Marshal(reopened)
	if !bytes.Equal(got, want) {
		t.Fatal("reopening changed saved coordinates or orders")
	}
	call(t, readOnly, "GET", "/scenarios/"+first.ScenarioID, nil, 200, &reopened)
	if reopened.Snapshot.Revision != created.Snapshot.Revision || reopened.CurrentPlanID == nil || *reopened.CurrentPlanID != *run.PlanID {
		t.Fatal("switching changed an existing plan")
	}
}
