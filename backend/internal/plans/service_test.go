package plans

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestBuildMatchesPublishedBackendExample(t *testing.T) {
	example := loadBackendExample(t)
	var snapshot contracts.Snapshot
	var request contracts.BuildPlanRequest
	var matrix contracts.TravelMatrix
	var solveResult contracts.SolveResult
	var geometry contracts.RoutesGeometry
	var expected contracts.PlanResult
	decodeExample(t, example, "snapshot", &snapshot)
	decodeExample(t, example, "build_request", &request)
	decodeExample(t, example, "matrix", &matrix)
	decodeExample(t, example, "solve_result", &solveResult)
	decodeExample(t, example, "routes_geometry", &geometry)
	decodeExample(t, example, "plan_result", &expected)

	geo := &fakeGeo{
		matrixFn: func(input contracts.MatrixRequest) (contracts.TravelMatrix, error) { return matrix, nil },
		routesFn: func(input contracts.RoutesRequest) (contracts.RoutesGeometry, error) { return geometry, nil },
	}
	planner := &fakePlanner{solveFn: func(input contracts.SolveRequest) (contracts.SolveResult, error) { return solveResult, nil }}
	service := mustService(t, &fakeData{snapshot: snapshot}, geo, planner)

	actual, err := service.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Build result differs from published example\nactual:   %#v\nexpected: %#v", actual, expected)
	}
}

func TestReplanMatchesPublishedCancellationExample(t *testing.T) {
	example := loadBackendExample(t)
	var snapshot contracts.Snapshot
	var base contracts.Plan
	var request contracts.ReplanRequest
	var expected contracts.PlanResult
	decodeExample(t, example, "snapshot", &snapshot)
	decodeExample(t, example, "saved_plan", &base)
	decodeExample(t, example, "replan_request", &request)
	decodeExample(t, example, "replan_result", &expected)
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, &fakeGeo{}, &fakePlanner{solveFn: solveFirstOrder})

	actual, err := service.Replan(context.Background(), request)
	if err != nil {
		t.Fatalf("Replan returned error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Replan result differs from published example\nactual:   %#v\nexpected: %#v", actual, expected)
	}
}

type fakeData struct {
	snapshot contracts.Snapshot
	plan     contracts.Plan
}

func (fake *fakeData) GetSnapshot(_ context.Context, scenarioID string, revision int64) (contracts.Snapshot, error) {
	if fake.snapshot.ScenarioID != scenarioID || fake.snapshot.Revision != revision {
		return contracts.Snapshot{}, contracts.NewError(contracts.ErrorNotFound, "snapshot not found", nil)
	}
	return cloneSnapshot(fake.snapshot), nil
}

func (fake *fakeData) GetPlan(_ context.Context, planID string) (contracts.Plan, error) {
	if fake.plan.ID != planID {
		return contracts.Plan{}, contracts.NewError(contracts.ErrorNotFound, "plan not found", nil)
	}
	return fake.plan, nil
}

type fakeGeo struct {
	geocodeFn     func(contracts.GeocodeRequest) (contracts.GeocodeResult, error)
	matrixFn      func(contracts.MatrixRequest) (contracts.TravelMatrix, error)
	routesFn      func(contracts.RoutesRequest) (contracts.RoutesGeometry, error)
	positionFn    func(contracts.PositionRequest) (contracts.PositionResult, error)
	matrixCalls   int
	routesCalls   int
	positionCalls int
}

func (fake *fakeGeo) Geocode(_ context.Context, input contracts.GeocodeRequest) (contracts.GeocodeResult, error) {
	if fake.geocodeFn == nil {
		return contracts.GeocodeResult{}, errors.New("unexpected Geocode call")
	}
	return fake.geocodeFn(input)
}

func (fake *fakeGeo) BuildMatrix(_ context.Context, input contracts.MatrixRequest) (contracts.TravelMatrix, error) {
	fake.matrixCalls++
	if fake.matrixFn == nil {
		return contracts.TravelMatrix{}, errors.New("unexpected BuildMatrix call")
	}
	return fake.matrixFn(input)
}

func (fake *fakeGeo) BuildRoutes(_ context.Context, input contracts.RoutesRequest) (contracts.RoutesGeometry, error) {
	fake.routesCalls++
	if fake.routesFn == nil {
		return contracts.RoutesGeometry{}, errors.New("unexpected BuildRoutes call")
	}
	return fake.routesFn(input)
}

func (fake *fakeGeo) PositionAt(_ context.Context, input contracts.PositionRequest) (contracts.PositionResult, error) {
	fake.positionCalls++
	if fake.positionFn == nil {
		return contracts.PositionResult{}, errors.New("unexpected PositionAt call")
	}
	return fake.positionFn(input)
}

type fakePlanner struct {
	solveFn func(contracts.SolveRequest) (contracts.SolveResult, error)
	modes   []contracts.SolveMode
}

func (fake *fakePlanner) Solve(_ context.Context, input contracts.SolveRequest) (contracts.SolveResult, error) {
	fake.modes = append(fake.modes, input.Mode)
	return fake.solveFn(input)
}

func TestBuildReturnsValidatedPlanWithBaselineAndGeometry(t *testing.T) {
	snapshot := testSnapshot()
	geo := standardGeo()
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot}, geo, planner)

	result, err := service.Build(context.Background(), contracts.BuildPlanRequest{RequestID: "request-1", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !reflect.DeepEqual(planner.modes, []contracts.SolveMode{contracts.SolveModeBaseline, contracts.SolveModeOptimized}) {
		t.Fatalf("unexpected planner modes: %v", planner.modes)
	}
	if geo.matrixCalls != 1 || geo.routesCalls != 1 {
		t.Fatalf("unexpected geo calls: matrix=%d routes=%d", geo.matrixCalls, geo.routesCalls)
	}
	if result.Draft.SnapshotRevision != 1 || result.Draft.BasePlanID != nil || result.AppliedEvent != nil {
		t.Fatalf("unexpected build result identity: %+v", result.Draft)
	}
	if result.Draft.BaselineMetrics == nil || result.Draft.BaselineMetrics.AssignedCount != 1 {
		t.Fatalf("baseline metrics were not populated: %+v", result.Draft.BaselineMetrics)
	}
	if result.Draft.Metrics.AssignedCount != 1 || result.Draft.Metrics.TotalDistanceM != 1200 {
		t.Fatalf("unexpected metrics: %+v", result.Draft.Metrics)
	}
	if len(result.Draft.Routes) != 1 || len(result.Draft.Routes[0].Legs[0].Geometry) != 2 {
		t.Fatalf("route geometry was not attached: %+v", result.Draft.Routes)
	}
	if result.Draft.AsOf != mustTime("2026-09-16T21:00:00Z") {
		t.Fatalf("unexpected as_of: %s", result.Draft.AsOf)
	}
}

func TestBuildRejectsPlannerThatOmitsOrder(t *testing.T) {
	snapshot := testSnapshot()
	geo := standardGeo()
	planner := &fakePlanner{solveFn: func(input contracts.SolveRequest) (contracts.SolveResult, error) {
		return contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}, nil
	}}
	service := mustService(t, &fakeData{snapshot: snapshot}, geo, planner)

	_, err := service.Build(context.Background(), contracts.BuildPlanRequest{RequestID: "request-1", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision})
	requireContractCode(t, err, contracts.ErrorInvalidPlan)
}

func TestReplanCancelsFutureOrderWithoutCallingPlanner(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	geo := &fakeGeo{}
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)

	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "request-2", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, BasePlanID: base.ID,
		Event: contracts.Event{ID: "event-1", OccurredAt: mustTime("2026-09-17T05:00:00Z"), Type: contracts.EventOrderCancelled, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", Reason: "client_refusal"})},
	})
	if err != nil {
		t.Fatalf("Replan returned error: %v", err)
	}
	if len(planner.modes) != 0 || geo.matrixCalls != 0 || geo.routesCalls != 0 {
		t.Fatalf("empty future should not call planner or route building")
	}
	if result.TargetSnapshot.Revision != 2 || result.TargetSnapshot.Orders[0].Status != contracts.OrderStatusCancelled {
		t.Fatalf("event was not applied to target snapshot: %+v", result.TargetSnapshot)
	}
	if !reflect.DeepEqual(result.Draft.CancelledOrderIDs, []string{"order-1"}) || len(result.Draft.Routes) != 0 {
		t.Fatalf("unexpected cancelled plan: %+v", result.Draft)
	}
	if len(result.Draft.Changes) != 1 || result.Draft.Changes[0].Reason != contracts.PlanChangeCancelled || result.Draft.Changes[0].Before == nil || result.Draft.Changes[0].After != nil {
		t.Fatalf("unexpected changes: %+v", result.Draft.Changes)
	}
}

func TestReplanUnavailableEngineerSplitsActiveLeg(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Orders[0].Status = contracts.OrderStatusEnRoute
	snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: ptr(mustTime("2026-09-17T06:00:00Z"))}
	base := testSavedPlan(snapshot)
	geo := &fakeGeo{positionFn: func(input contracts.PositionRequest) (contracts.PositionResult, error) {
		return contracts.PositionResult{
			Point:              contracts.Point{Lat: 55.755, Lon: 37.61},
			ElapsedDurationSec: 450,
			ElapsedDistanceM:   600,
			ElapsedGeometry:    []contracts.Point{{Lat: 55.75, Lon: 37.61}, {Lat: 55.755, Lon: 37.61}},
		}, nil
	}}
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)

	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "request-2", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, BasePlanID: base.ID,
		Event: contracts.Event{ID: "event-2", OccurredAt: mustTime("2026-09-17T06:07:30Z"), Type: contracts.EventEngineerUnavailable, Payload: contracts.EncodePayload(contracts.EventPayload{EngineerID: "eng-1"})},
	})
	if err != nil {
		t.Fatalf("Replan returned error: %v", err)
	}
	if geo.positionCalls != 1 || len(planner.modes) != 0 {
		t.Fatalf("unexpected dependency calls: position=%d planner=%v", geo.positionCalls, planner.modes)
	}
	if len(result.Draft.Routes) != 1 || len(result.Draft.Routes[0].Legs) != 2 || result.Draft.Routes[0].Legs[0].DistanceM != 600 || len(result.Draft.Routes[0].Visits) != 1 {
		t.Fatalf("active leg was not preserved correctly: %+v", result.Draft.Routes)
	}
	if result.Draft.Metrics.TotalDistanceM != 1200 || result.Draft.Metrics.UnassignedCount != 0 {
		t.Fatalf("unexpected full-day metrics: %+v", result.Draft.Metrics)
	}
	if len(result.TargetSnapshot.Locations) != 3 || result.TargetSnapshot.Engineers[0].Available {
		t.Fatalf("target snapshot was not updated: %+v", result.TargetSnapshot)
	}
	if len(result.Draft.Changes) != 0 {
		t.Fatalf("unexpected changes: %+v", result.Draft.Changes)
	}
}

func TestReplanAddsUrgentOrderAndReturnsResolvedEvent(t *testing.T) {
	snapshot := testSnapshot()
	snapshot.Orders = []contracts.Order{}
	base := contracts.Plan{ID: "plan-0", PlanDraft: contracts.PlanDraft{
		ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, AsOf: mustTime("2026-09-16T21:00:00Z"), Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, CancelledOrderIDs: []string{}, Issues: []contracts.Issue{}, Changes: []contracts.PlanChange{}, Termination: contracts.TerminationCompleted,
	}}
	geo := standardGeo()
	geo.geocodeFn = func(input contracts.GeocodeRequest) (contracts.GeocodeResult, error) {
		point := contracts.Point{Lat: 55.76, Lon: 37.61}
		return contracts.GeocodeResult{Items: []contracts.GeocodeItem{{LocationID: "loc-2", Location: &contracts.Location{ID: "loc-2", Address: input.Locations[0].Address, Point: point}}}}, nil
	}
	planner := &fakePlanner{solveFn: solveFirstOrder}
	service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, geo, planner)
	transport := contracts.TransportCar
	newOrder := contracts.Order{ID: "order-2", LocationID: "loc-2", RequiredSkills: []string{"repair"}, RequiredTransport: &transport, Window: contracts.Window{Start: mustTime("2026-09-17T07:00:00Z"), End: mustTime("2026-09-17T09:00:00Z")}, ServiceSec: 1800}
	newOrder.ServiceSec = 4800
	newOrder.WorkType = contracts.WorkTypeEmergency
	newOrder.Priority = contracts.PriorityUrgent
	newOrder.Status = contracts.OrderStatusActive
	newOrder.ReceivedAt = mustTime("2026-09-17T05:00:00Z")

	result, err := service.Replan(context.Background(), contracts.ReplanRequest{
		RequestID: "request-3", ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, BasePlanID: base.ID,
		Event: contracts.Event{ID: "event-3", OccurredAt: mustTime("2026-09-17T05:00:00Z"), Type: contracts.EventUrgentOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &newOrder, Location: &contracts.LocationInput{ID: "loc-2", Address: "Москва, новый адрес"}})},
	})
	if err != nil {
		t.Fatalf("Replan returned error: %v", err)
	}
	if !reflect.DeepEqual(planner.modes, []contracts.SolveMode{contracts.SolveModeOptimized}) {
		t.Fatalf("unexpected planner calls: %v", planner.modes)
	}
	added := result.TargetSnapshot.Orders[0]
	if added.Priority != contracts.PriorityUrgent || added.Status != contracts.OrderStatusActive || added.SourceOrder != 1 {
		t.Fatalf("urgent order was not normalized: %+v", added)
	}
	if result.AppliedEvent == nil || contracts.DecodePayload(result.AppliedEvent.Payload).Location == nil || contracts.DecodePayload(result.AppliedEvent.Payload).Location.Point == nil {
		t.Fatalf("applied event does not contain resolved coordinates: %+v", result.AppliedEvent)
	}
	if len(result.Draft.Changes) != 1 || result.Draft.Changes[0].Reason != contracts.PlanChangeAssigned {
		t.Fatalf("unexpected changes: %+v", result.Draft.Changes)
	}
}

func mustService(t *testing.T, data PlanDataReader, geo GeoService, planner Planner) *Service {
	t.Helper()
	service, err := New(data, geo, planner, Options{TimeLimitMS: 1000})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return service
}

func standardGeo() *fakeGeo {
	return &fakeGeo{
		matrixFn: func(input contracts.MatrixRequest) (contracts.TravelMatrix, error) {
			return completeMatrix(input.Locations, input.Profiles, "geo-1"), nil
		},
		routesFn: func(input contracts.RoutesRequest) (contracts.RoutesGeometry, error) {
			locations := make(map[string]contracts.Point, len(input.Locations))
			for _, location := range input.Locations {
				locations[location.ID] = location.Point
			}
			items := make([]contracts.RouteGeometry, 0, len(input.Legs))
			for _, leg := range input.Legs {
				items = append(items, contracts.RouteGeometry{LegID: leg.LegID, Geometry: []contracts.Point{locations[leg.FromLocationID], locations[leg.ToLocationID]}})
			}
			return contracts.RoutesGeometry{Items: items}, nil
		},
	}
}

func completeMatrix(locations []contracts.Location, profiles []contracts.Transport, geoContextID string) contracts.TravelMatrix {
	ids := make([]string, len(locations))
	for index, location := range locations {
		ids[index] = location.ID
	}
	matrix := contracts.TravelMatrix{ID: "matrix-1", GeoContextID: geoContextID, LocationIDs: ids, Profiles: make(map[contracts.Transport][][]contracts.TravelCell)}
	for _, profile := range profiles {
		rows := make([][]contracts.TravelCell, len(ids))
		for from := range ids {
			rows[from] = make([]contracts.TravelCell, len(ids))
			for to := range ids {
				duration, distance := int64(0), int64(0)
				if from != to {
					duration, distance = 900, 1200
				}
				rows[from][to] = contracts.TravelCell{Reachable: true, DurationSec: &duration, DistanceM: &distance}
			}
		}
		matrix.Profiles[profile] = rows
	}
	return matrix
}

func solveFirstOrder(input contracts.SolveRequest) (contracts.SolveResult, error) {
	if len(input.Orders) == 0 {
		return contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}, nil
	}
	order := input.Orders[0]
	engineer := input.Engineers[0]
	state := input.EngineerStates[0]
	cell, err := matrixCell(input.TravelMatrix, engineer.Transport, state.StartLocationID, order.LocationID)
	if err != nil {
		return contracts.SolveResult{}, err
	}
	legStart := state.AvailableFrom
	arrival := legStart.Add(time.Duration(*cell.DurationSec) * time.Second)
	workStart := arrival
	if order.Window.Start.After(workStart) {
		workStart = order.Window.Start
	}
	visit := contracts.Visit{OrderID: order.ID, ArrivalAt: arrival, StartAt: workStart, EndAt: workStart.Add(time.Duration(order.ServiceSec) * time.Second)}
	leg := contracts.Leg{ID: "leg-1", FromLocationID: state.StartLocationID, ToLocationID: order.LocationID, StartAt: legStart, EndAt: arrival, DistanceM: *cell.DistanceM, GeoContextID: input.TravelMatrix.GeoContextID}
	return contracts.SolveResult{Routes: []contracts.Route{{EngineerID: engineer.ID, StartLocationID: state.StartLocationID, StartAt: state.AvailableFrom, Visits: []contracts.Visit{visit}, Legs: []contracts.Leg{leg}}}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}, nil
}

func testSnapshot() contracts.Snapshot {
	transport := contracts.TransportCar
	return contracts.Snapshot{
		ScenarioID: "scenario-1", Revision: 1, RegionID: "region-1", Date: "2026-09-17", Timezone: "Europe/Moscow", OfficeLocationID: "office-1",
		Locations: []contracts.Location{{ID: "office-1", Address: "Москва, офис", Point: contracts.Point{Lat: 55.75, Lon: 37.61}}, {ID: "loc-1", Address: "Москва, ул. Тверская, 1", Point: contracts.Point{Lat: 55.76, Lon: 37.61}}},
		Orders:    []contracts.Order{{ID: "order-1", LocationID: "loc-1", RequiredSkills: []string{"repair"}, RequiredTransport: &transport, Window: contracts.Window{Start: mustTime("2026-09-17T07:00:00Z"), End: mustTime("2026-09-17T09:00:00Z")}, ServiceSec: 1800, Priority: contracts.PriorityNormal, SourceOrder: 1, Status: contracts.OrderStatusActive}},
		Engineers: []contracts.Engineer{{ID: "eng-1", Skills: []string{"repair"}, Transport: contracts.TransportCar, Shift: contracts.Window{Start: mustTime("2026-09-17T06:00:00Z"), End: mustTime("2026-09-17T15:00:00Z")}, Available: true, SourceOrder: 1}},
		Issues:    []contracts.Issue{},
	}
}

func testSavedPlan(snapshot contracts.Snapshot) contracts.Plan {
	leg := contracts.Leg{ID: "leg-1", FromLocationID: "office-1", ToLocationID: "loc-1", StartAt: mustTime("2026-09-17T06:00:00Z"), EndAt: mustTime("2026-09-17T06:15:00Z"), DistanceM: 1200, GeoContextID: "geo-1", Geometry: []contracts.Point{snapshot.Locations[0].Point, snapshot.Locations[1].Point}}
	visit := contracts.Visit{OrderID: "order-1", ArrivalAt: mustTime("2026-09-17T06:15:00Z"), StartAt: mustTime("2026-09-17T07:00:00Z"), EndAt: mustTime("2026-09-17T07:30:00Z")}
	return contracts.Plan{ID: "plan-1", PlanDraft: contracts.PlanDraft{ScenarioID: snapshot.ScenarioID, SnapshotRevision: 1, AsOf: mustTime("2026-09-16T21:00:00Z"), Routes: []contracts.Route{{EngineerID: "eng-1", StartLocationID: "office-1", StartAt: mustTime("2026-09-17T06:00:00Z"), Visits: []contracts.Visit{visit}, Legs: []contracts.Leg{leg}}}, Unassigned: []contracts.UnassignedOrder{}, CancelledOrderIDs: []string{}, Issues: []contracts.Issue{}, Metrics: contracts.Metrics{AssignedCount: 1, UsedEngineerCount: 1, TotalDistanceM: 1200, PerEngineer: []contracts.EngineerDistance{{EngineerID: "eng-1", DistanceM: 1200}}}, Changes: []contracts.PlanChange{}, Termination: contracts.TerminationCompleted}}
}

func mustTime(value string) time.Time {
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return result
}

func requireContractCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", code)
	}
	var contractError *contracts.ContractError
	if !errors.As(err, &contractError) || contractError.Code != code {
		t.Fatalf("expected %s ContractError, got %T %v", code, err, err)
	}
}

func loadBackendExample(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "docs", "contracts", "examples", "backend_flow.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read backend example: %v", err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode backend example: %v", err)
	}
	return result
}

func decodeExample(t *testing.T, example map[string]json.RawMessage, key string, target any) {
	t.Helper()
	raw, exists := example[key]
	if !exists {
		t.Fatalf("backend example key %q is missing", key)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode backend example %q: %v", key, err)
	}
}
