//go:build vroom

package optimized_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/optimized"
)

func TestAuditSecondBoundariesAndEmptyInputs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*c.SolveRequest)
		assigned int
	}{
		{"exact-window-and-shift", func(in *c.SolveRequest) {}, 1},
		{"window-one-second-too-early", func(in *c.SolveRequest) {
			in.Orders[0].Window.Start = in.Orders[0].Window.Start.Add(-time.Second)
			in.Orders[0].Window.End = in.Orders[0].Window.End.Add(-time.Second)
		}, 0},
		{"shift-one-second-too-short", func(in *c.SolveRequest) { in.Engineers[0].Shift.End = in.Engineers[0].Shift.End.Add(-time.Second) }, 0},
		{"received-one-second-later", func(in *c.SolveRequest) { in.Orders[0].ReceivedAt = in.Orders[0].ReceivedAt.Add(time.Second) }, 0},
		{"available-one-second-later", func(in *c.SolveRequest) {
			in.EngineerStates[0].AvailableFrom = in.EngineerStates[0].AvailableFrom.Add(time.Second)
		}, 0},
		{"finish-after-window-is-allowed", func(in *c.SolveRequest) { in.Orders[0].Window.Start = in.Orders[0].Window.Start.Add(-time.Minute) }, 1},
		{"no-workers", func(in *c.SolveRequest) { in.Engineers = nil; in.EngineerStates = nil }, 0},
		{"no-jobs", func(in *c.SolveRequest) { in.Orders = nil }, 0},
		{"no-jobs-and-workers", func(in *c.SolveRequest) { in.Orders = nil; in.Engineers = nil; in.EngineerStates = nil }, 0},
		{"unreachable", func(in *c.SolveRequest) { in.TravelMatrix.Profiles[c.TransportCar][0][1] = c.TravelCell{} }, 0},
		{"zero-travel-same-location", func(in *c.SolveRequest) {
			in.Orders[0].LocationID = "depot"
			in.Orders[0].Window.Start = testutil.At(6, 0)
			in.Orders[0].Window.End = testutil.At(6, 0)
		}, 1},
		{"instant-shift", func(in *c.SolveRequest) { in.Engineers[0].Shift.End = in.Engineers[0].Shift.Start }, 0},
		{"available-after-shift", func(in *c.SolveRequest) { in.EngineerStates[0].AvailableFrom = testutil.At(19, 0) }, 0},
		{"leap-day-midnight-offset", func(in *c.SolveRequest) {
			base := time.Date(2024, 2, 29, 23, 55, 0, 0, time.FixedZone("+0545", 20700))
			in.Engineers[0].Shift = c.Window{Start: base, End: base.Add(20 * time.Minute)}
			in.EngineerStates[0].AvailableFrom = base
			in.Orders[0].ReceivedAt = base
			in.Orders[0].Window = c.Window{Start: base.Add(10 * time.Minute), End: base.Add(10 * time.Minute)}
		}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := testutil.BaseRequest()
			in.Mode = c.SolveModeOptimized
			in.TimeLimitMS = 100
			in.Orders[0].Window = c.Window{Start: testutil.At(6, 10), End: testutil.At(6, 10)}
			in.Orders[0].ReceivedAt = testutil.At(6, 0)
			in.Engineers[0].Shift.End = testutil.At(6, 20)
			tc.change(&in)
			opt, possible := auditExact(in)
			out, err := optimized.New().Solve(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			got := auditResult(t, in, out, possible)
			if got != opt || len(in.Orders)-len(out.Unassigned) != tc.assigned {
				t.Fatalf("got %v optimum %v output %+v", got, opt, out)
			}
		})
	}
}

// Fuzz invalid API-domain values before they reach the native solver. This is
// separate from random feasible-instance enumeration and does not treat
// arbitrary malformed JSON as proof of routing correctness.
func FuzzPlannerRejectsInvalidInput(f *testing.F) {
	for i := uint8(0); i < 24; i++ {
		f.Add(i, int64(i))
	}
	f.Fuzz(func(t *testing.T, kind uint8, value int64) {
		in := testutil.BaseRequest()
		in.Mode = c.SolveModeOptimized
		switch kind % 24 {
		case 0:
			in.TimeLimitMS = 0
		case 1:
			in.TimeLimitMS = math.MaxInt64
		case 2:
			in.Orders[0].ServiceSec = 0
		case 3:
			in.Orders[0].ServiceSec = math.MaxInt64
		case 4:
			in.Orders[0].ReceivedAt = time.Time{}
		case 5:
			in.Orders[0].Window.End = in.Orders[0].Window.Start.Add(-time.Second)
		case 6:
			in.Orders = append(in.Orders, in.Orders[0])
		case 7:
			in.Engineers = append(in.Engineers, in.Engineers[0])
		case 8:
			in.EngineerStates = nil
		case 9:
			in.EngineerStates = append(in.EngineerStates, in.EngineerStates[0])
		case 10:
			in.TravelMatrix.Profiles[c.TransportCar][0][1].DurationSec = nil
		case 11:
			in.TravelMatrix.Profiles[c.TransportCar][0][1].DistanceM = testutil.Ptr(-1)
		case 12:
			in.TravelMatrix.Profiles[c.TransportCar][0][0] = c.TravelCell{}
		case 13:
			in.TravelMatrix.Profiles[c.TransportCar][0] = nil
		case 14:
			in.Engineers[0].Available = false
		case 15:
			in.EngineerStates[0].EquipmentAvailable[c.EquipmentRouter] = 3
		case 16:
			in.Orders[0].EquipmentRequired[c.EquipmentRouter] = -1
		case 17:
			in.Orders[0].WorkType = c.WorkTypeEmergency
			in.Orders[0].Priority = c.PriorityNormal
		case 18:
			in.Orders[0].Status = c.OrderStatusInProgress
		case 19:
			in.EngineerStates[0].StartLocationID = "unknown"
		case 20:
			in.Orders[0].ReceivedAt = in.Orders[0].ReceivedAt.Add(time.Nanosecond)
		case 21:
			in.TravelMatrix.Profiles[c.TransportCar][0][1].DurationSec = testutil.Ptr(-1)
		case 22:
			in.AlreadyUsedEngineerIDs = []string{"e1", "e1"}
		case 23:
			in.Orders[0].LocationID = "unknown"
		}
		// Vary a still-valid scalar independently of the invalid field.
		in.Orders[0].SourceOrder = value
		_, err := optimized.New().Solve(context.Background(), in)
		var contract *c.ContractError
		if !errors.As(err, &contract) || contract.Code != "INVALID_INPUT" {
			t.Fatalf("invalid case %d accepted: %v", kind%24, err)
		}
	})
}
