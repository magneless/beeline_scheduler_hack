package optimized

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// SearchOptions enables independently measurable search experiments. Zero values
// select the original search; New enables the hybrid configuration. Options and
// diagnostics belong to one solve call.
type SearchOptions struct {
	Seed            int64
	FastEstimate    bool
	CrewElimination bool
	Segments        bool
	Adaptive        bool
	RegretOnly      bool
	// RefineAfterRepair reserves the last 30% of repair time for enhanced moves.
	RefineAfterRepair bool
}

type OperatorDiagnostics struct {
	Attempts     int64 `json:"attempts"`
	Improvements int64 `json:"improvements"`
	ElapsedMS    int64 `json:"elapsed_ms"`
}

type SearchDiagnostics struct {
	Estimates           int64                          `json:"estimates"`
	Iterations          int64                          `json:"iterations"`
	Improvements        int64                          `json:"improvements"`
	ConstructorCounts   [3]int64                       `json:"constructor_counts"`
	ConstructorCrews    int64                          `json:"constructor_crews"`
	ConstructorDistance int64                          `json:"constructor_distance_m"`
	Operators           map[string]OperatorDiagnostics `json:"operators,omitempty"`
}

func NewWithSearchOptions(options SearchOptions) *Optimized {
	o := New()
	o.search = options
	return o
}

// SolveWithDiagnostics shares exactly the production solver and constraints.
// Measurements never influence feasibility or the incumbent acceptance rule.
func (o *Optimized) SolveWithDiagnostics(ctx context.Context, in contracts.SolveRequest) (contracts.SolveResult, SearchDiagnostics, error) {
	diagnostics := SearchDiagnostics{Operators: map[string]OperatorDiagnostics{}}
	result, err := o.solveMeasured(ctx, in, &diagnostics)
	return result, diagnostics, err
}
