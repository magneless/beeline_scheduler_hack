package plans

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

func (service *Service) Build(ctx context.Context, input contracts.BuildPlanRequest) (contracts.PlanResult, error) {
	if input.OptionKey == "late_emergency" {
		strict, err := service.buildWithPolicy(ctx, input, dispatchPolicy{key: "strict"})
		if err != nil {
			return contracts.PlanResult{}, err
		}
		eligible := lateEligible(strict, nil)
		if len(eligible) == 0 {
			strict.Draft.OptionKey = "late_emergency"
			return strict, nil
		}
		return service.buildWithPolicy(ctx, input, dispatchPolicy{key: "late_emergency", lateIDs: eligible})
	}
	if input.OptionKey != "" && input.OptionKey != "strict" {
		return contracts.PlanResult{}, contracts.InvalidInput("unsupported initial plan option", nil)
	}
	return service.buildWithPolicy(ctx, input, dispatchPolicy{key: "strict"})
}

func (service *Service) buildWithPolicy(ctx context.Context, input contracts.BuildPlanRequest, policy dispatchPolicy) (contracts.PlanResult, error) {
	progress.Report(ctx, "preparing", "Читаем заявки и бригады")
	if err := ctx.Err(); err != nil {
		return contracts.PlanResult{}, err
	}
	if input.RequestID == "" || input.ScenarioID == "" || input.SnapshotRevision <= 0 {
		return contracts.PlanResult{}, contracts.InvalidInput("request_id, scenario_id and positive snapshot_revision are required", nil)
	}

	mode, err := service.resolveMode(input.SolveMode, nil)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	snapshot, err := service.data.GetSnapshot(ctx, input.ScenarioID, input.SnapshotRevision)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("get snapshot", err)
	}
	if err := validateSnapshot(snapshot, input.ScenarioID, input.SnapshotRevision); err != nil {
		return contracts.PlanResult{}, err
	}

	orders := activeValidOrders(snapshot)
	if policy.key == "late_emergency" {
		orders = widenEmergencyWindows(orders, policy.lateIDs, snapshot.Engineers)
	}
	for _, o := range snapshot.Orders {
		if o.Execution != nil {
			return contracts.PlanResult{}, contracts.EventConflict("use Replan after execution starts", nil)
		}
	}
	if input.ExpectedCurrentPlanID != nil {
		previous, e := service.data.GetPlan(ctx, *input.ExpectedCurrentPlanID)
		if e != nil {
			return contracts.PlanResult{}, e
		}
		mode, e = service.resolveMode(input.SolveMode, &previous)
		if e != nil {
			return contracts.PlanResult{}, e
		}
		day, _, e := localDayBounds(snapshot)
		if e != nil {
			return contracts.PlanResult{}, e
		}
		if previous.BasePlanID != nil || previous.AsOf.After(day) {
			return contracts.PlanResult{}, contracts.EventConflict("use Replan after events", nil)
		}
	}
	// A from-scratch comparison includes the original day's pool, including
	// crews that became reserve after accepting the first plan.
	planningSnapshot := cloneSnapshot(snapshot)
	for i := range planningSnapshot.Engineers {
		if snapshot.ReserveInitialized && planningSnapshot.Engineers[i].Reserve {
			planningSnapshot.Engineers[i].Available = true
		}
	}
	engineers := availableEngineers(planningSnapshot)
	states := make([]contracts.EngineerState, 0, len(engineers))
	for _, engineer := range engineers {
		states = append(states, contracts.EngineerState{
			EngineerID:         engineer.ID,
			StartLocationID:    snapshot.OfficeLocationID,
			AvailableFrom:      engineer.Shift.Start,
			EquipmentAvailable: cloneEquipment(engineer.EquipmentStock),
		})
	}
	locations, err := relevantLocations(snapshot, orders, states)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	profiles := profilesFor(engineers)
	if len(engineers) == 0 {
		result, err := service.buildWithoutEngineers(snapshot, orders, mode)
		if err == nil {
			result.Draft.OptionKey = input.OptionKey
			if input.OptionKey != "" {
				result.Draft.DeferredOrderIDs = deferredOrderIDs(result.Draft.Unassigned)
			}
		}
		return result, err
	}
	progress.Report(ctx, "matrix", "Получаем время в пути")
	matrix, err := service.geo.BuildMatrix(ctx, contracts.MatrixRequest{Locations: locations, Profiles: profiles, GeoContextID: nil})
	if err != nil {
		return contracts.PlanResult{}, dependencyError("build travel matrix", err)
	}
	if err := validateMatrix(matrix, locations, profiles); err != nil {
		return contracts.PlanResult{}, err
	}

	baseRequest := contracts.SolveRequest{
		EmergencyFirst:         policy.key == "late_emergency",
		Orders:                 orders,
		Engineers:              engineers,
		EngineerStates:         states,
		AlreadyUsedEngineerIDs: []string{},
		TravelMatrix:           matrix,
		TimeLimitMS:            service.timeLimitMS,
	}
	baselineRequest := cloneSolveRequest(baseRequest)
	baselineRequest.Mode = contracts.SolveModeBaseline
	progress.Report(ctx, "solving", "Рассчитываем базовый маршрут")
	baseline, err := service.planner.Solve(ctx, baselineRequest)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("solve baseline plan", err)
	}
	if err := validateSolveResult(baselineRequest, baseline); err != nil {
		return contracts.PlanResult{}, err
	}

	optimizedRequest := cloneSolveRequest(baseRequest)
	optimizedRequest.Mode = mode
	progress.Report(ctx, "solving", "Рассчитываем маршрут")
	optimized, err := service.planner.Solve(ctx, optimizedRequest)
	if err != nil {
		return contracts.PlanResult{}, dependencyError("solve optimized plan", err)
	}
	if err := validateSolveResult(optimizedRequest, optimized); err != nil {
		return contracts.PlanResult{}, err
	}
	progress.Report(ctx, "geometry", "Загружаем маршруты на карту")
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
		OptionKey:         input.OptionKey,
		SolveMode:         mode,
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
		CompletedOrderIDs: []string{},
	}
	draft.EquipmentRemaining, err = equipmentRemaining(snapshot)
	draft.Issues = append(draft.Issues, service.issues...)
	if err != nil {
		return contracts.PlanResult{}, err
	}
	for _, o := range snapshot.Orders {
		if o.Status == contracts.OrderStatusCancelled {
			draft.CancelledOrderIDs = append(draft.CancelledOrderIDs, o.ID)
		}
	}
	if mode == contracts.SolveModeBaseline {
		draft.Issues = append(draft.Issues, contracts.Issue{Code: "BASELINE_ONLY", Message: "Выбран базовый алгоритм планирования"})
	}
	if policy.key == "late_emergency" {
		draft.Lateness = measureLateness(snapshot, routes)
		if err := validateFinalPlan(snapshot, routes, unassigned, draft.CancelledOrderIDs, draft.Lateness); err != nil {
			return contracts.PlanResult{}, err
		}
	}
	if input.OptionKey != "" {
		draft.DeferredOrderIDs = deferredOrderIDs(unassigned)
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
