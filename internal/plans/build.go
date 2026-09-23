package plans

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/contracts"
)

func (service *Service) Build(ctx context.Context, input contracts.BuildPlanRequest) (contracts.PlanResult, error) {
	if err := ctx.Err(); err != nil {
		return contracts.PlanResult{}, err
	}
	if input.RequestID == "" || input.ScenarioID == "" || input.SnapshotRevision <= 0 {
		return contracts.PlanResult{}, contracts.InvalidInput("request_id, scenario_id and positive snapshot_revision are required", nil)
	}

	snapshot, err := service.data.GetSnapshot(ctx, input.ScenarioID, input.SnapshotRevision)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("get snapshot", err)
	}
	if err := validateSnapshot(snapshot, input.ScenarioID, input.SnapshotRevision); err != nil {
		return contracts.PlanResult{}, err
	}

	orders := activeValidOrders(snapshot)
	engineers := availableEngineers(snapshot)
	states := make([]contracts.EngineerState, 0, len(engineers))
	for _, engineer := range engineers {
		states = append(states, contracts.EngineerState{
			EngineerID:      engineer.ID,
			StartLocationID: snapshot.OfficeLocationID,
			AvailableFrom:   engineer.Shift.Start,
		})
	}
	locations, err := relevantLocations(snapshot, orders, states)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	profiles := profilesFor(engineers)
	matrix, err := service.geo.BuildMatrix(ctx, contracts.MatrixRequest{Locations: locations, Profiles: profiles, GeoContextID: nil})
	if err != nil {
		return contracts.PlanResult{}, dependencyError("build travel matrix", err)
	}
	if err := validateMatrix(matrix, locations, profiles); err != nil {
		return contracts.PlanResult{}, err
	}

	baseRequest := contracts.SolveRequest{
		Orders:                 orders,
		Engineers:              engineers,
		EngineerStates:         states,
		AlreadyUsedEngineerIDs: []string{},
		TravelMatrix:           matrix,
		TimeLimitMS:            service.timeLimitMS,
	}
	baselineRequest := cloneSolveRequest(baseRequest)
	baselineRequest.Mode = contracts.SolveModeBaseline
	baseline, err := service.planner.Solve(ctx, baselineRequest)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("solve baseline plan", err)
	}
	if err := validateSolveResult(baselineRequest, baseline); err != nil {
		return contracts.PlanResult{}, err
	}

	optimizedRequest := cloneSolveRequest(baseRequest)
	optimizedRequest.Mode = contracts.SolveModeOptimized
	optimized, err := service.planner.Solve(ctx, optimizedRequest)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("solve optimized plan", err)
	}
	if err := validateSolveResult(optimizedRequest, optimized); err != nil {
		return contracts.PlanResult{}, err
	}
	routes, err := service.addGeometry(ctx, optimized.Routes, engineers, locations, matrix.GeoContextID)
	if err != nil {
		return contracts.PlanResult{}, err
	}

	asOf, _, err := localDayBounds(snapshot)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	baselineMetrics := calculateMetrics(baseline.Routes, baseline.Unassigned)
	unassigned := nonNil(append([]contracts.UnassignedOrder(nil), optimized.Unassigned...))
	draft := contracts.PlanDraft{
		ScenarioID:        snapshot.ScenarioID,
		SnapshotRevision:  snapshot.Revision,
		BasePlanID:        nil,
		AsOf:              asOf,
		Routes:            nonNil(routes),
		Unassigned:        unassigned,
		CancelledOrderIDs: []string{},
		Issues:            nonNil(append([]contracts.Issue(nil), snapshot.Issues...)),
		Metrics:           calculateMetrics(routes, unassigned),
		BaselineMetrics:   &baselineMetrics,
		Changes:           []contracts.PlanChange{},
		Termination:       optimized.Termination,
	}
	return contracts.PlanResult{
		Draft:          draft,
		TargetSnapshot: cloneSnapshot(snapshot),
		AppliedEvent:   nil,
	}, nil
}

func (service *Service) addGeometry(ctx context.Context, routes []contracts.Route, engineers []contracts.Engineer, locations []contracts.Location, geoContextID string) ([]contracts.Route, error) {
	if len(routes) == 0 {
		return []contracts.Route{}, nil
	}
	request, err := routeGeometryRequest(routes, engineerMap(engineers), locations, geoContextID)
	if err != nil {
		return nil, err
	}
	if len(request.Legs) == 0 {
		return nonNil(routes), nil
	}
	geometry, err := service.geo.BuildRoutes(ctx, request)
	if err != nil {
		return nil, dependencyError("build route geometry", err)
	}
	return attachGeometry(routes, geometry)
}
