package optimized

import (
	"context"
	"errors"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func (o *Optimized) solveRouting(ctx context.Context, input contracts.SolveRequest, deadline time.Time) (contracts.SolveResult, error) {
	return o.solveRoutingMeasured(ctx, input, deadline, nil)
}

func (o *Optimized) solveRoutingMeasured(ctx context.Context, input contracts.SolveRequest, deadline time.Time, diagnostics *SearchDiagnostics) (contracts.SolveResult, error) {
	p, err := prepareRouting(input)
	if err != nil {
		return contracts.SolveResult{}, err
	}
	p.search, p.diagnostics = o.search, diagnostics
	if p.search.FastEstimate {
		p.prepareFastEstimate()
	}
	best := p.emptyResult()
	if len(p.orders) == 0 || len(p.workers) == 0 {
		if err := p.explain(ctx, &best, deadline); err != nil {
			return contracts.SolveResult{}, err
		}
		if !time.Now().Before(deadline) {
			best.Termination = contracts.TerminationTimeLimit
		}
		if err := ctx.Err(); err != nil {
			return contracts.SolveResult{}, err
		}
		return best, nil
	}
	if p.search.RegretOnly {
		best, err = p.repairSearch(ctx, best, deadline.Add(-20*time.Millisecond))
		if err != nil {
			return contracts.SolveResult{}, err
		}
		if err = p.explain(ctx, &best, deadline); err != nil {
			return contracts.SolveResult{}, err
		}
		best.Termination = contracts.TerminationTimeLimit
		return best, nil
	}
	binary, err := o.executable()
	if err != nil {
		return contracts.SolveResult{}, err
	}
	// Port of the qualified VROOM + repair constructor: roughly 40% of the
	// total budget goes to native candidates, the remainder to domain search.
	constructorDeadline := deadline.Add(-(time.Duration(input.TimeLimitMS) * time.Millisecond / 10 * 6))
	native, err := p.vroomRequest(ctx)
	if err != nil {
		return contracts.SolveResult{}, err
	}
	variants := []vroomInput{native}
	if tight, changed := p.earlyEmergencyWindows(native); changed {
		variants = []vroomInput{tight, native}
	}
	for i, variant := range variants {
		if err := ctx.Err(); err != nil {
			return contracts.SolveResult{}, err
		}
		remaining := time.Until(constructorDeadline)
		if remaining < time.Millisecond {
			break
		}
		candidate, err := p.constructVROOM(ctx, binary, variant, time.Now().Add(remaining/time.Duration(len(variants)-i)))
		if err != nil {
			return contracts.SolveResult{}, err
		}
		if betterPlan(p.score(candidate), p.score(best), input.EmergencyFirst) {
			best = candidate
		}
	}
	if diagnostics != nil {
		s := p.score(best)
		diagnostics.ConstructorCounts, diagnostics.ConstructorCrews = s.counts, s.used
		diagnostics.ConstructorDistance = s.distance
	}
	// Feasible route sequences, never VROOM's proposed times, cross the domain
	// boundary. Each local move is accepted only by the exact public objective.
	searchDeadline := deadline.Add(-min(20*time.Millisecond, max(0, time.Until(deadline))/20))
	best, err = p.repairSearch(ctx, best, searchDeadline)
	if err != nil {
		return contracts.SolveResult{}, err
	}
	if err := p.explain(ctx, &best, deadline); err != nil {
		return contracts.SolveResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return contracts.SolveResult{}, err
	}
	best.Termination = contracts.TerminationTimeLimit // Bounded heuristic, no optimality proof.
	return best, nil
}

func (p *routingProblem) earlyEmergencyWindows(input vroomInput) (vroomInput, bool) {
	tight := input
	tight.Jobs = append([]vroomJob(nil), input.Jobs...)
	changed := false
	for j, job := range tight.Jobs {
		idx := job.ID - 1
		order := p.orders[idx]
		if order.WorkType != contracts.WorkTypeEmergency {
			continue
		}
		end := job.TimeWindows[0][1]
		for v := range p.workers {
			estimate := p.estimate(v, []int{idx})
			if estimate.ok {
				end = min(end, estimate.finish.Unix()-order.ServiceSec-p.origin)
			}
		}
		if end < job.TimeWindows[0][1] {
			tight.Jobs[j].TimeWindows = [][2]int64{{job.TimeWindows[0][0], end}}
			changed = true
		}
	}
	return tight, changed
}

func (p *routingProblem) constructVROOM(ctx context.Context, binary string, input vroomInput, deadline time.Time) (contracts.SolveResult, error) {
	classes := []int{}
	for class := 0; class < 3; class++ {
		for _, job := range input.Jobs {
			if workClass(p.orders[job.ID-1]) == class {
				classes = append(classes, class)
				break
			}
		}
	}
	best := p.emptyResult()
	pinned := map[int]bool{}
	for i, class := range classes {
		if err := ctx.Err(); err != nil {
			return contracts.SolveResult{}, err
		}
		remaining := time.Until(deadline)
		if remaining < time.Millisecond {
			break
		}
		stage := input
		stage.Jobs = []vroomJob{}
		for _, job := range input.Jobs {
			if workClass(p.orders[job.ID-1]) != class && !pinned[job.ID] {
				continue
			}
			if pinned[job.ID] {
				job.Priority = 100 // Search hint, not a lexicographic guarantee.
			}
			stage.Jobs = append(stage.Jobs, job)
		}
		data, err := runVROOM(ctx, binary, stage, time.Now().Add(remaining/time.Duration(len(classes)-i)))
		if ctx.Err() != nil {
			return contracts.SolveResult{}, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			continue // Keep the last validated candidate on the internal timeout.
		}
		if err != nil {
			return contracts.SolveResult{}, err
		}
		sequences, err := p.vroomSequences(stage, data)
		if err != nil {
			return contracts.SolveResult{}, err
		}
		// VROOM cannot express departure >= received_at of the next job.
		// Drop visits made infeasible by that rule and re-evaluate every leg,
		// including shortcuts after removals on asymmetric road matrices.
		candidate, err := p.materialize(p.feasiblePrefixes(sequences))
		if err != nil {
			return contracts.SolveResult{}, err
		}
		if betterPlan(p.score(candidate), p.score(best), p.input.EmergencyFirst) {
			best = candidate
		}
		pinned = map[int]bool{}
		for _, route := range best.Routes {
			for _, visit := range route.Visits {
				pinned[p.orderIndex[visit.OrderID]+1] = true
			}
		}
	}
	return best, nil
}
