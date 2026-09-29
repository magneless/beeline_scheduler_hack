// Package planner dispatches scheduling modes through a common contract.
package planner

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/baseline"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/shared"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/optimized"
)

// Planner dispatches all contract modes. Each call owns its search state.
type Planner struct{}

var _ contracts.Planner = (*Planner)(nil)

func New() *Planner { return &Planner{} }
func (*Planner) Solve(ctx context.Context, input contracts.SolveRequest) (contracts.SolveResult, error) {
	switch input.Mode {
	case contracts.SolveModeBaseline:
		return NewBaseline().Solve(ctx, input)
	case contracts.SolveModeOptimized:
		return NewOptimized().Solve(ctx, input)
	case contracts.SolveModeInsertOnly:
		return NewInserter().Solve(ctx, input)
	default:
		return contracts.SolveResult{}, shared.InvalidInput("mode", "неизвестный режим планировщика")
	}
}

// Baseline is the source-order planner implemented by package baseline.
type Baseline = baseline.Baseline

// Optimized is the VROOM planner implemented by package optimized.
type Optimized = optimized.Optimized

// NewBaseline preserves the planner facade constructor.
func NewBaseline() *Baseline { return baseline.New() }

// NewOptimized preserves the planner facade constructor.
func NewOptimized() *Optimized { return optimized.New() }
