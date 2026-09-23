package plans

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/contracts"
)

type PlanDataReader interface {
	GetSnapshot(ctx context.Context, scenarioID string, revision int64) (contracts.Snapshot, error)
	GetPlan(ctx context.Context, planID string) (contracts.Plan, error)
}

type GeoService interface {
	Geocode(ctx context.Context, input contracts.GeocodeRequest) (contracts.GeocodeResult, error)
	BuildMatrix(ctx context.Context, input contracts.MatrixRequest) (contracts.TravelMatrix, error)
	BuildRoutes(ctx context.Context, input contracts.RoutesRequest) (contracts.RoutesGeometry, error)
	PositionAt(ctx context.Context, input contracts.PositionRequest) (contracts.PositionResult, error)
}

type Planner interface {
	Solve(ctx context.Context, input contracts.SolveRequest) (contracts.SolveResult, error)
}

type PlanService interface {
	Build(ctx context.Context, input contracts.BuildPlanRequest) (contracts.PlanResult, error)
	Replan(ctx context.Context, input contracts.ReplanRequest) (contracts.PlanResult, error)
}

type Options struct {
	TimeLimitMS int64
	Mode        contracts.SolveMode
	Issues      []contracts.Issue
}

type Service struct {
	data        PlanDataReader
	geo         GeoService
	planner     Planner
	timeLimitMS int64
	mode        contracts.SolveMode
	issues      []contracts.Issue
}

var _ PlanService = (*Service)(nil)

func New(data PlanDataReader, geo GeoService, planner Planner, options Options) (*Service, error) {
	if data == nil {
		return nil, contracts.InvalidInput("PlanDataReader is required", map[string]any{"dependency": "data"})
	}
	if geo == nil {
		return nil, contracts.InvalidInput("GeoService is required", map[string]any{"dependency": "geo"})
	}
	if planner == nil {
		return nil, contracts.InvalidInput("Planner is required", map[string]any{"dependency": "planner"})
	}
	if options.TimeLimitMS <= 0 {
		options.TimeLimitMS = 1000
	}
	if options.Mode == "" {
		options.Mode = contracts.SolveModeOptimized
	}
	if options.Mode != contracts.SolveModeBaseline && options.Mode != contracts.SolveModeOptimized {
		return nil, contracts.InvalidInput("invalid solve mode", nil)
	}
	return &Service{data: data, geo: geo, planner: planner, timeLimitMS: options.TimeLimitMS, mode: options.Mode, issues: append([]contracts.Issue{}, options.Issues...)}, nil
}
