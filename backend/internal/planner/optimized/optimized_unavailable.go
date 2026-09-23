//go:build !ortools

package optimized

import (
	"context"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func solveRouting(ctx context.Context, input contracts.SolveRequest, deadline time.Time) (contracts.SolveResult, error) {
	return contracts.SolveResult{}, computationError("optimized требует сборки с -tags ortools и нативной библиотеки Airspace OR-Tools; см. backend/README.md")
}
