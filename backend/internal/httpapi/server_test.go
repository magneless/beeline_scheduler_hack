package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/geo"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/experiment"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/plans"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/runs"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

// Legacy contract tests exercise the original run API with the real planner.
// The wrapper intentionally exposes only PlanService to the HTTP layer.
type legacyPlanService struct{ c.PlanService }

func integrationServer(t *testing.T, integrated ...bool) (http.Handler, *storage.Store, testkit.Fixture) {
	return integrationServerMode(t, c.SolveModeBaseline, integrated...)
}

func integrationServerMode(t *testing.T, solverMode c.SolveMode, integrated ...bool) (http.Handler, *storage.Store, testkit.Fixture) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for HTTP/PostgreSQL integration tests")
	}
	admin, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "http_" + strings.ReplaceAll(storage.ID("test"), "-", "_")
	if _, e = admin.Exec("CREATE SCHEMA " + schema); e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(dsn)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := storage.Open(context.Background(), u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close(); admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	if e = s.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	f, e := testkit.Load("../../../docs/contracts/examples/backend_flow.json")
	if e != nil {
		t.Fatal(e)
	}
	api := &Server{Store: s, Importer: &data.Importer{Geo: testkit.Geocoder{}, Root: "../../../datasets/original", Prepared: map[string]c.Snapshot{"contract-example": f.Snapshot}}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var planService c.PlanService = &testkit.Plans{Reader: s, Fixture: f}
	if len(integrated) > 0 && integrated[0] {
		g := geo.NewGeoService(&geo.DemoProvider{})
		api.Importer.Geo = g
		var solver c.Planner = planner.New()
		if os.Getenv("PLANNER_EXPERIMENT_URL") != "" {
			solver = experiment.FromEnvironment()
		}
		service, err := plans.New(s, g, solver, plans.Options{TimeLimitMS: 1000, Mode: solverMode})
		if err != nil {
			t.Fatal(err)
		}
		planService = service
	}
	api.Plans = planService
	if len(integrated) > 0 && integrated[0] && (len(integrated) < 2 || !integrated[1]) {
		api.Plans = legacyPlanService{PlanService: planService}
	}
	w := runs.Worker{Store: s, Plans: planService, Timeout: time.Second * 5}
	go func() { defer close(done); w.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return api.Handler(), s, f
}
func call(t *testing.T, h http.Handler, method, path string, in any, status int, out any) {
	t.Helper()
	var body io.Reader
	if raw, ok := in.(string); ok {
		body = strings.NewReader(raw)
	} else if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			t.Fatal(e)
		}
		body = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, "/api/v1"+path, body)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: want %d got %d: %s", method, path, status, w.Code, w.Body.String())
	}
	if out != nil {
		if e := json.Unmarshal(w.Body.Bytes(), out); e != nil {
			t.Fatal(e)
		}
	}
}
func awaitRun(t *testing.T, h http.Handler, id string) c.Run {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		var r c.Run
		call(t, h, "GET", "/runs/"+id, nil, 200, &r)
		if r.Status == "succeeded" || r.Status == "failed" {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run timed out")
	return c.Run{}
}
func TestHTTPScenarioBuildEventsAndHistory(t *testing.T) {
	h, _, f := integrationServer(t)
	var catalog struct{ Items []data.Dataset }
	call(t, h, "GET", "/demo-datasets", nil, 200, &catalog)
	if len(catalog.Items) != 4 {
		t.Fatal(catalog)
	}
	var v c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &v)
	sid := v.Snapshot.ScenarioID
	var accepted map[string]string
	build := map[string]any{"request_id": "build", "snapshot_revision": 1, "expected_current_plan_id": nil}
	call(t, h, "POST", "/scenarios/"+sid+"/plans", build, 202, &accepted)
	run := awaitRun(t, h, accepted["run_id"])
	if run.Status != "succeeded" {
		t.Fatalf("build: %+v", run)
	}
	var p c.Plan
	call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &p)
	original := p.ID
	var repeat map[string]string
	call(t, h, "POST", "/scenarios/"+sid+"/plans", build, 202, &repeat)
	if repeat["run_id"] != run.ID {
		t.Fatal("repeat changed run")
	}
	for n, step := range f.Execution.Steps {
		body := map[string]any{"request_id": step.Request.RequestID, "snapshot_revision": n + 1, "event": step.Request.Event}
		call(t, h, "POST", "/plans/"+p.ID+"/events", body, 202, &accepted)
		run = awaitRun(t, h, accepted["run_id"])
		if run.Status != "succeeded" {
			t.Fatalf("step %d: %+v", n, run)
		}
		call(t, h, "GET", "/plans/"+*run.PlanID, nil, 200, &p)
	}
	if p.Metrics.CompletedCount != 1 || p.SnapshotRevision != 5 || p.EquipmentRemaining["eng-1"][c.EquipmentRouter] != 0 {
		t.Fatalf("wrong final plan %+v", p)
	}
	call(t, h, "GET", "/scenarios/"+sid+"?revision=1", nil, 200, &v)
	if v.Snapshot.Orders[0].Status != c.OrderStatusActive || *v.CurrentPlanID != p.ID {
		t.Fatal("historical view corrupted")
	}
	call(t, h, "GET", "/plans/"+original, nil, 200, nil)
	call(t, h, "PATCH", "/scenarios/"+sid+"/engineers/eng-1", map[string]any{"expected_revision": 5, "available": false}, 409, nil)
	call(t, h, "POST", "/scenarios/"+sid+"/plans", map[string]any{"request_id": "blocked", "snapshot_revision": 5, "expected_current_plan_id": p.ID}, 409, nil)
	// Unsupported stub event is accepted by infrastructure; its failure is visible through Run.
	event := c.Event{ID: "unavailable", OccurredAt: p.AsOf, Type: "engineer_unavailable", Payload: json.RawMessage(`{"engineer_id":"eng-1"}`)}
	call(t, h, "POST", "/plans/"+p.ID+"/events", map[string]any{"request_id": "unavailable", "snapshot_revision": 5, "event": event}, 202, &accepted)
	failed := awaitRun(t, h, accepted["run_id"])
	if failed.Status != "failed" || failed.Error == nil || failed.PlanID != nil {
		t.Fatal(failed)
	}
	call(t, h, "GET", "/scenarios/"+sid, nil, 200, &v)
	if v.Snapshot.Revision != 5 || *v.CurrentPlanID != p.ID {
		t.Fatal("failed event changed scenario")
	}
}
func TestHTTPImportPatchAndErrors(t *testing.T) {
	h, _, _ := integrationServer(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, e := mw.CreateFormFile("file", "orders.csv")
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.Open("../../../datasets/original/Восток Синтетические данные.csv")
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(part, f)
	f.Close()
	mw.WriteField("region_id", "east")
	mw.WriteField("date", "2026-08-17")
	mw.Close()
	r := httptest.NewRequest("POST", "/api/v1/scenarios/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v c.ScenarioView
	json.Unmarshal(w.Body.Bytes(), &v)
	if len(v.Snapshot.Engineers) != 0 {
		t.Fatal("CSV import fabricated engineers")
	}
	body.Reset()
	mw = multipart.NewWriter(&body)
	part, e = mw.CreateFormFile("file", "engineers.csv")
	if e != nil {
		t.Fatal(e)
	}
	io.WriteString(part, "id;skills;transport;shift_start;shift_end;available;router;tv_box\ncrew-1;repair;car;08:00;18:00;true;2;1\n")
	mw.WriteField("expected_revision", "1")
	mw.Close()
	r = httptest.NewRequest("POST", "/api/v1/scenarios/"+v.Snapshot.ScenarioID+"/engineers/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Snapshot.Engineers) != 1 || v.Snapshot.Revision != 2 {
		t.Fatal(v)
	}
	path := "/scenarios/" + v.Snapshot.ScenarioID + "/engineers/" + v.Snapshot.Engineers[0].ID
	call(t, h, "PATCH", path, `{"expected_revision":2,"skills":null}`, 422, nil)
	call(t, h, "PATCH", path, `{"expected_revision":2,"shift":{"start":"2026-08-17T08:00:00Z"}}`, 422, nil)
	call(t, h, "PATCH", path, `{"expected_revision":2,"shift":{"start":"2026-08-17T08:00:00Z","end":"2026-08-17T09:00:00Z","unknown":true}}`, 422, nil)
	call(t, h, "PATCH", path, `{"expected_revision":2,"skills":[]}`, 200, &v)
	if v.Snapshot.Revision != 3 || len(v.Snapshot.Engineers[0].Skills) != 0 {
		t.Fatal(v)
	}
	call(t, h, "PATCH", path, `{"expected_revision":1,"available":false}`, 409, nil)
	call(t, h, "POST", "/scenarios", `{`, 400, nil)
	call(t, h, "POST", "/scenarios", `{"demo_dataset_id":42}`, 422, nil)
	call(t, h, "GET", "/scenarios/missing", nil, 404, nil)
	call(t, h, "GET", "/scenarios/"+v.Snapshot.ScenarioID+"?revision=0", nil, 422, nil)
	call(t, h, "POST", "/scenarios/"+v.Snapshot.ScenarioID+"/plans", `{"request_id":"x","snapshot_revision":2}`, 422, nil)
}
func TestValidateAllEventTypes(t *testing.T) {
	at := time.Date(2026, 8, 17, 7, 0, 0, 0, time.UTC)
	cases := []struct {
		kind, payload string
		valid         bool
	}{{"engineer_unavailable", `{"engineer_id":"eng"}`, true}, {"order_cancelled", `{"order_id":"o","reason":"client_refusal"}`, true}, {"order_cancelled", `{"order_id":"o","reason":"anything"}`, false}, {"order_status_changed", `{"order_id":"o","engineer_id":"eng","status":"in_progress"}`, true}, {"order_status_changed", `{"order_id":"o","engineer_id":"eng","status":"completed","expected_end_at":"2026-08-17T09:00:00Z"}`, false}, {"urgent_order_added", `{"order":{"id":"u","location_id":"loc","work_type":"emergency","required_skills":["emergency"],"window":{"start":"2026-08-17T07:00:00Z","end":"2026-08-17T08:00:00Z"},"received_at":"2026-08-17T07:00:00Z","service_sec":4800,"priority":"urgent","equipment_required":{},"status":"active","execution":null},"location":null}`, true}}
	for _, tt := range cases {
		t.Run(tt.kind+tt.payload, func(t *testing.T) {
			e := validateEvent(c.Event{ID: "e", OccurredAt: at, Type: tt.kind, Payload: json.RawMessage(tt.payload)})
			if (e == nil) != tt.valid {
				t.Fatal(e)
			}
		})
	}
}
