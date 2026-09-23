package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestHTTPOrdinaryAndUrgentOrdersWithIntegratedPlanner(t *testing.T) {
	h, _, _ := integrationServer(t, true)
	var view c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &view)
	sid := view.Snapshot.ScenarioID
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+sid+"/plans", map[string]any{"request_id": "initial", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("initial build: %+v", run)
	}
	var plan c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &plan)
	if len(plan.Routes) == 0 || len(plan.Routes[0].Visits) == 0 {
		t.Fatal("initial plan has no visits")
	}
	originalVisits := make(map[string]c.Visit)
	for _, route := range plan.Routes {
		for _, visit := range route.Visits {
			originalVisits[visit.OrderID] = visit
		}
	}

	at := plan.AsOf.UTC()
	ordinary := map[string]any{
		"order": map[string]any{
			"id": "http-ordinary", "location_id": view.Snapshot.Orders[0].LocationID,
			"work_type": "repair", "required_skills": []string{}, "required_transport": nil,
			"window":      map[string]any{"start": at, "end": at.Add(20 * time.Hour)},
			"received_at": at, "service_sec": 900, "priority": "normal",
			"equipment_required": map[string]int64{}, "status": "active", "execution": nil,
		},
	}
	event := map[string]any{"id": "ordinary-http", "occurred_at": at, "type": "ordinary_order_added", "payload": ordinary}
	request := map[string]any{"request_id": "ordinary-http", "snapshot_revision": 1, "event": event}
	call(t, h, "POST", "/plans/"+plan.ID+"/events", request, 202, &accepted)
	ordinaryRun := awaitRun(t, h, accepted["run_id"])
	if ordinaryRun.Status != "succeeded" {
		t.Fatalf("ordinary event: %+v", ordinaryRun)
	}
	var after c.Plan
	call(t, h, "GET", "/plans/"+*ordinaryRun.PlanID, nil, 200, &after)
	if after.SnapshotRevision != 2 {
		t.Fatalf("ordinary event revision = %d", after.SnapshotRevision)
	}
	currentVisits := make(map[string]c.Visit)
	for _, route := range after.Routes {
		for _, visit := range route.Visits {
			currentVisits[visit.OrderID] = visit
		}
	}
	for id, before := range originalVisits {
		afterVisit, ok := currentVisits[id]
		if !ok || afterVisit.ArrivalAt != before.ArrivalAt || afterVisit.StartAt != before.StartAt || afterVisit.EndAt != before.EndAt {
			t.Fatalf("ordinary insertion changed existing visit %s: before=%+v after=%+v", id, before, afterVisit)
		}
	}
	var repeated map[string]string
	call(t, h, "POST", "/plans/"+plan.ID+"/events", request, 202, &repeated)
	if repeated["run_id"] != ordinaryRun.ID {
		t.Fatalf("idempotent event created another run: %v vs %v", repeated["run_id"], ordinaryRun.ID)
	}

	urgentAt := after.AsOf.UTC()
	urgent := map[string]any{
		"order": map[string]any{
			"id": "http-urgent", "location_id": "http-urgent-location", "work_type": "emergency",
			"required_skills": []string{}, "required_transport": "walk",
			"window":      map[string]any{"start": urgentAt, "end": urgentAt.Add(20 * time.Hour)},
			"received_at": urgentAt, "service_sec": 4800, "priority": "urgent",
			"equipment_required": map[string]int64{}, "status": "active", "execution": nil,
		},
		"location": map[string]any{"id": "http-urgent-location", "address": "Москва, Тверская улица, 13"},
	}
	urgentEvent := map[string]any{"id": "urgent-http", "occurred_at": urgentAt, "type": "urgent_order_added", "payload": urgent}
	call(t, h, "POST", "/plans/"+after.ID+"/events", map[string]any{"request_id": "urgent-http", "snapshot_revision": 2, "event": urgentEvent}, 202, &accepted)
	urgentRun := awaitRun(t, h, accepted["run_id"])
	if urgentRun.Status != "succeeded" {
		t.Fatalf("urgent event: %+v", urgentRun)
	}
	var urgentView c.ScenarioView
	call(t, h, "GET", "/scenarios/"+sid, nil, 200, &urgentView)
	var urgentLocation c.Location
	for _, location := range urgentView.Snapshot.Locations {
		if location.ID == "http-urgent-location" {
			urgentLocation = location
		}
	}
	if urgentLocation.Address != "Москва, Тверская улица, 13" || urgentLocation.Point.Lat == 0 || urgentLocation.Point.Lon == 0 {
		t.Fatalf("urgent location was not geocoded and persisted: %+v", urgentLocation)
	}
}

func TestHTTPRosterReplacementInvalidatesPlanAndBlocksAfterExecution(t *testing.T) {
	h, _, _ := integrationServer(t, true)
	var view c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &view)
	sid := view.Snapshot.ScenarioID
	var accepted map[string]string
	call(t, h, "POST", "/scenarios/"+sid+"/plans", map[string]any{"request_id": "roster-build", "snapshot_revision": 1, "expected_current_plan_id": nil}, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("build: %+v", run)
	}
	oldPlan := *run.PlanID
	var roster c.ScenarioView
	postRoster(t, h, sid, 1, "crew-replacement;repair;car;08:00;18:00;true;2;1\n", http.StatusOK, &roster)
	if roster.Snapshot.Revision != 2 || len(roster.Snapshot.Engineers) != 1 || roster.CurrentPlanID != nil {
		t.Fatalf("roster replacement did not invalidate plan: %+v", roster)
	}
	call(t, h, "GET", "/plans/"+oldPlan, nil, 200, nil)
	call(t, h, "POST", "/scenarios/"+sid+"/plans", map[string]any{"request_id": "roster-rebuild", "snapshot_revision": 2, "expected_current_plan_id": nil}, 202, &accepted)
	run = awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("rebuild: %+v", run)
	}
	var rebuilt c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &rebuilt)
	if len(rebuilt.Routes) == 0 || rebuilt.Routes[0].EngineerID != "crew-replacement" {
		t.Fatalf("rebuild used wrong roster: %+v", rebuilt.Routes)
	}
	// Once an execution event is accepted, replacing the roster must be rejected.
	at := rebuilt.AsOf.UTC()
	e := map[string]any{"id": "sent", "occurred_at": at, "type": "order_status_changed", "payload": map[string]any{"order_id": rebuilt.Routes[0].Visits[0].OrderID, "engineer_id": "crew-replacement", "status": "sent"}}
	call(t, h, "POST", "/plans/"+rebuilt.ID+"/events", map[string]any{"request_id": "sent", "snapshot_revision": 2, "event": e}, 202, &accepted)
	run = awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("execution event: %+v", run)
	}
	postRoster(t, h, sid, 3, "crew-late;repair;car;08:00;18:00;true;1;0\n", http.StatusConflict, nil)
}

func postRoster(t *testing.T, h http.Handler, sid string, revision int64, csv string, wantStatus int, out *c.ScenarioView) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "engineers.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(part, "id;skills;transport;shift_start;shift_end;available;router;tv_box\n"+csv); err != nil {
		t.Fatal(err)
	}
	if err = mw.WriteField("expected_revision", strconv.FormatInt(revision, 10)); err != nil {
		t.Fatal(err)
	}
	if err = mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v1/scenarios/"+sid+"/engineers/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != wantStatus {
		t.Fatalf("roster import: want %d got %d: %s", wantStatus, w.Code, w.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
}
