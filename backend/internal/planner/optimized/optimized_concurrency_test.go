//go:build ortools

package optimized_test

import (
	"context"
	"fmt"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/optimized"
	"sync"
	"testing"
)

func TestOptimizedConcurrentModelsRemainIndependent(t *testing.T) {
	solver := optimized.New()
	var group sync.WaitGroup
	failures := make(chan error, 4)
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			in := testutil.BaseRequest()
			in.Mode = contracts.SolveModeOptimized
			in.TimeLimitMS = 100
			in.Orders[0].ID = fmt.Sprintf("private-%d", i)
			in.Orders[0].ReceivedAt = testutil.At(8+i, 0)
			got, err := solver.Solve(context.Background(), in)
			if err != nil {
				failures <- err
				return
			}
			if len(got.Routes) != 1 || len(got.Routes[0].Visits) != 1 || got.Routes[0].Visits[0].OrderID != in.Orders[0].ID || got.Routes[0].Legs[0].StartAt.Before(in.Orders[0].ReceivedAt) {
				failures <- fmt.Errorf("model %d mixed or lost data: %+v", i, got)
			}
		}(i)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
