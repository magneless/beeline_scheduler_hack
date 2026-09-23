package planner

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

func TestTimeLimitKeepsEveryOrder(t *testing.T) {
	data, err := os.ReadFile("../../docs/contracts/examples/backend_flow.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name       string
		expireCall int
		assigned   int
	}{
		{"before_first_assignment", 2, 0},
		{"during_engineer_scan", 3, 0},
		{"after_first_assignment", 4, 1},
		{"after_last_assignment", 6, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var fixture struct {
				Request contracts.SolveRequest `json:"solve_request"`
			}
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatal(err)
			}
			input := fixture.Request
			input.Mode = contracts.SolveModeBaseline
			input.TimeLimitMS = 1
			second := input.Orders[0]
			second.ID = "order-2"
			second.SourceOrder++
			input.Orders = append(input.Orders, second)
			input.Engineers[0].EquipmentStock[contracts.EquipmentRouter] = 2
			input.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] = 2
			base := time.Now()
			calls := 0
			solver := &Baseline{now: func() time.Time {
				calls++
				if calls >= tt.expireCall {
					return base.Add(time.Millisecond)
				}
				return base
			}}
			got, err := solver.Solve(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Termination != contracts.TerminationTimeLimit {
				t.Fatalf("termination=%s, want time_limit", got.Termination)
			}
			seen := map[string]int{}
			assigned := 0
			for _, route := range got.Routes {
				for _, visit := range route.Visits {
					seen[visit.OrderID]++
					assigned++
				}
			}
			for _, item := range got.Unassigned {
				seen[item.OrderID]++
				if item.ReasonCode != contracts.ReasonNotAssignedBySolver {
					t.Fatalf("timeout must not claim infeasibility: %+v", item)
				}
			}
			if assigned != tt.assigned || len(seen) != 2 || seen["order-1"] != 1 || seen["order-2"] != 1 {
				t.Fatalf("assigned=%d, coverage=%v; want %d assignments and both orders once", assigned, seen, tt.assigned)
			}
		})
	}
}
