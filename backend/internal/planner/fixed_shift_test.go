//go:build vroom

package planner

import (
	"context"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

func TestMandatoryShiftBoundsTravelAndWorkInBothSolvers(t *testing.T) {
	for _, mode := range []c.SolveMode{c.SolveModeBaseline, c.SolveModeOptimized} {
		t.Run(string(mode), func(t *testing.T) {
			in := testutil.BaseRequest()
			in.Mode = mode
			shift, err := c.WorkingShift("2026-09-17", "Europe/Moscow")
			if err != nil {
				t.Fatal(err)
			}
			in.Engineers[0].Shift = shift
			// The state can precede 10:00; it must never permit an early departure.
			in.EngineerStates[0].AvailableFrom = testutil.At(5, 0)
			in.Orders = []c.Order{
				testutil.Order("before-shift", "p1", 1, 6, 0, 10, 0),
				testutil.Order("morning", "p1", 2, 6, 0, 10, 0),
				testutil.Order("ends-at-22", "p1", 3, 18, 50, 10, 0),
				testutil.Order("ends-after-22", "p1", 4, 18, 51, 10, 0),
			}
			in.Orders[0].Window.End = testutil.At(6, 59)
			in.Orders[1].Window.End = testutil.At(7, 10)
			in.Orders[2].Window.End = testutil.At(18, 50)
			in.Orders[3].Window.End = testutil.At(19, 0)
			got, err := New().Solve(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			assigned := testutil.AssignedIDs(got)
			if len(assigned) != 2 || assigned["morning"] == "" || assigned["ends-at-22"] == "" {
				t.Fatalf("wrong boundary assignments: %+v", got)
			}
			for _, route := range got.Routes {
				for _, leg := range route.Legs {
					if leg.StartAt.Before(shift.Start) || leg.EndAt.After(shift.End) {
						t.Fatalf("travel outside shift: %+v", leg)
					}
				}
				for _, visit := range route.Visits {
					if visit.StartAt.Before(shift.Start) || visit.EndAt.After(shift.End) {
						t.Fatalf("work outside shift: %+v", visit)
					}
				}
			}
		})
	}
}
