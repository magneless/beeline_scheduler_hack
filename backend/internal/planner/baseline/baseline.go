// Package baseline builds schedules by appending orders in source order.
package baseline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/shared"
)

// Baseline appends orders to the first feasible engineer in source order.
// It is safe for concurrent calls; each call owns its routing state.
// The zero value is ready to use. Only mode=baseline is implemented.
type Baseline struct {
	now func() time.Time // Test clock; nil uses the monotonic wall clock.
}

var _ contracts.Planner = (*Baseline)(nil)

// New returns a baseline planner.
func New() *Baseline { return &Baseline{} }

// Solve never mutates input. A search timeout returns a complete, feasible
// partial assignment with termination=time_limit. Cancellation of the caller's
// context returns ctx.Err(); that result must not be used as a plan.
func (b *Baseline) Solve(ctx context.Context, input contracts.SolveRequest) (contracts.SolveResult, error) {
	now := b.now
	if now == nil {
		now = time.Now
	}
	started := now()
	if err := ctx.Err(); err != nil {
		return contracts.SolveResult{}, err
	}
	if input.Mode != contracts.SolveModeBaseline {
		return contracts.SolveResult{}, shared.InvalidInput("mode", "Baseline поддерживает только baseline; используйте planner.New() для выбора режима")
	}
	if err := shared.ValidateInput(ctx, input); err != nil {
		return contracts.SolveResult{}, err
	}
	deadline := started.Add(time.Duration(input.TimeLimitMS) * time.Millisecond)
	stop := func() (bool, error) {
		if err := ctx.Err(); err != nil {
			return true, err
		}
		return !now().Before(deadline), nil
	}

	// Sort copies: source order is part of the contract, not slice/map order.
	orders := append([]contracts.Order(nil), input.Orders...)
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].SourceOrder != orders[j].SourceOrder {
			return orders[i].SourceOrder < orders[j].SourceOrder
		}
		return orders[i].ID < orders[j].ID
	})
	engineers := append([]contracts.Engineer(nil), input.Engineers...)
	sort.Slice(engineers, func(i, j int) bool {
		if engineers[i].SourceOrder != engineers[j].SourceOrder {
			return engineers[i].SourceOrder < engineers[j].SourceOrder
		}
		return engineers[i].ID < engineers[j].ID
	})
	locations := make(map[string]int, len(input.TravelMatrix.LocationIDs))
	for i, id := range input.TravelMatrix.LocationIDs {
		locations[id] = i
	}
	states := make(map[string]contracts.EngineerState, len(input.EngineerStates))
	for _, state := range input.EngineerStates {
		states[state.EngineerID] = state
	}
	workers := make([]shared.Worker, len(engineers))
	for i, engineer := range engineers {
		state := states[engineer.ID]
		start := shared.Later(engineer.Shift.Start, state.AvailableFrom).UTC()
		skills := make(map[string]bool, len(engineer.Skills))
		for _, skill := range engineer.Skills {
			skills[skill] = true
		}
		remaining := make(map[contracts.Equipment]int64, len(state.EquipmentAvailable))
		for equipment, count := range state.EquipmentAvailable {
			remaining[equipment] = count
		}
		workers[i] = shared.Worker{
			Engineer: engineer, Skills: skills, Remaining: remaining,
			Location: locations[state.StartLocationID], Time: start,
			InitialLocation: locations[state.StartLocationID],
			Route: contracts.Route{
				EngineerID: engineer.ID, StartLocationID: state.StartLocationID, StartAt: start,
				Visits: []contracts.Visit{}, Legs: []contracts.Leg{},
			},
		}
	}
	result := contracts.SolveResult{
		Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{},
		Termination: contracts.TerminationCompleted,
	}
	finish := func() contracts.SolveResult {
		for _, worker := range workers {
			if len(worker.Route.Visits) > 0 {
				result.Routes = append(result.Routes, worker.Route)
			}
		}
		return result
	}
	timedOut := func(next int) contracts.SolveResult {
		result.Termination = contracts.TerminationTimeLimit
		for _, order := range orders[next:] {
			result.Unassigned = append(result.Unassigned, shared.Unassigned(order, contracts.ReasonNotAssignedBySolver,
				"Расчёт остановлен по лимиту времени до назначения заявки."))
		}
		return finish()
	}
	legNumber := 0
	for i, order := range orders {
		if done, err := stop(); err != nil {
			return contracts.SolveResult{}, err
		} else if done {
			return timedOut(i), nil
		}
		assigned := false
		for j := range workers {
			if done, err := stop(); err != nil {
				return contracts.SolveResult{}, err
			} else if done {
				return timedOut(i), nil
			}
			worker := &workers[j]
			if !worker.MatchesSkills(order) || !worker.MatchesTransport(order) || !worker.HasEquipment(order) {
				continue
			}
			to := locations[order.LocationID]
			cell := input.TravelMatrix.Profiles[worker.Engineer.Transport][worker.Location][to]
			visit, leg, ok := worker.AppendCandidate(order, cell)
			if !ok {
				continue
			}
			legNumber++
			leg.ID = fmt.Sprintf("leg-%d", legNumber)
			leg.FromLocationID = input.TravelMatrix.LocationIDs[worker.Location]
			leg.GeoContextID = input.TravelMatrix.GeoContextID
			worker.Route.Visits = append(worker.Route.Visits, visit)
			worker.Route.Legs = append(worker.Route.Legs, leg)
			worker.Location, worker.Time = to, visit.EndAt
			for equipment, count := range order.EquipmentRequired {
				worker.Remaining[equipment] -= count
			}
			assigned = true
			break
		}
		if !assigned {
			reason, done, err := shared.ExplainUnassigned(order, workers, locations[order.LocationID], input.TravelMatrix, stop)
			if err != nil {
				return contracts.SolveResult{}, err
			}
			if done {
				return timedOut(i), nil
			}
			result.Unassigned = append(result.Unassigned, reason)
		}
	}
	if done, err := stop(); err != nil {
		return contracts.SolveResult{}, err
	} else if done {
		return timedOut(len(orders)), nil
	}
	return finish(), nil
}
