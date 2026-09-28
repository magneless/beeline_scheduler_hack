//go:build ortools

package optimized

import (
	"context"
	"fmt"
	"time"

	cs "github.com/airspacetechnologies/or-tools/go/ortools/constraintsolver"
	"github.com/airspacetechnologies/or-tools/go/ortools/util"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/baseline"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/shared"
	"google.golang.org/protobuf/types/known/durationpb"
)

func solveRouting(ctx context.Context, input contracts.SolveRequest, deadline time.Time) (result contracts.SolveResult, err error) {
	// Leave a bounded part of the budget for explaining unassigned work.
	searchDeadline := deadline.Add(-min(250*time.Millisecond, time.Until(deadline)/10))
	defer func() {
		if value := recover(); value != nil {
			result = contracts.SolveResult{}
			err = computationError(fmt.Sprintf("Ошибка Go API OR-Tools: %v", value))
		}
	}()
	p, err := prepareRouting(input)
	if err != nil {
		return result, err
	}
	best := p.emptyResult()
	score := p.score(best)
	if len(p.orders) == 0 || len(p.workers) == 0 {
		if err := p.explain(ctx, &best, deadline); err != nil {
			return result, err
		}
		if !time.Now().Before(searchDeadline) {
			best.Termination = contracts.TerminationTimeLimit
		}
		return best, nil
	}
	consider := func(candidate contracts.SolveResult) {
		s := p.score(candidate)
		if betterPlan(s, score, input.EmergencyFirst) {
			best, score = candidate, s
		}
	}
	// Baseline supplies an incumbent, retaining most of the budget for Routing.
	if remaining := time.Until(deadline); remaining > 2*time.Millisecond {
		seedInput := input
		seedInput.Mode = contracts.SolveModeBaseline
		seedInput.TimeLimitMS = max(int64(1), remaining.Milliseconds()/20)
		seed, seedErr := baseline.New().Solve(ctx, seedInput)
		if seedErr != nil {
			return result, seedErr
		}
		candidate, seedErr := p.materialize(p.sequences(seed))
		if seedErr != nil {
			return result, seedErr
		}
		consider(candidate)
	}
	limited := false
	for phase := 0; phase < 6; phase++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !time.Now().Before(deadline) {
			limited = true
			break
		}
		if p.phaseAtLowerBound(phase, score) {
			continue
		}
		budget := time.Until(searchDeadline) / time.Duration(6-phase)
		if budget < time.Millisecond {
			limited = true
			break
		}
		completed, phaseErr := p.searchPhase(ctx, phase, score, best, time.Now().Add(budget), consider)
		if phaseErr != nil {
			return result, phaseErr
		}
		limited = limited || !completed
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	best, err = p.fillAvailableSlots(ctx, best, time.Now().Add(time.Until(deadline)/2))
	if err != nil {
		return result, err
	}
	// Keep the native search and final insertion budgets unchanged. Spend only
	// half of the remaining diagnostic reserve on monotone route improvement.
	best, err = p.repairSearch(ctx, best, time.Now().Add(time.Until(deadline)/2))
	if err != nil {
		return result, err
	}
	if err := p.explain(ctx, &best, deadline); err != nil {
		return result, err
	}
	if limited || !time.Now().Before(deadline) {
		best.Termination = contracts.TerminationTimeLimit
	} else {
		best.Termination = contracts.TerminationCompleted
	}
	return best, nil
}
func (p *routingProblem) countClassForPhase(phase int) (int, bool) {
	if p.input.EmergencyFirst {
		switch phase {
		case 0:
			return 0, true
		case 2, 3:
			return phase - 1, true
		}
		return 0, false
	}
	return phase, phase < 3
}

func (p *routingProblem) delayPhase() int {
	if p.input.EmergencyFirst {
		return 1
	}
	return 3
}

func (p *routingProblem) phaseAtLowerBound(phase int, score planScore) bool {
	if class, ok := p.countClassForPhase(phase); ok {
		var total int64
		for _, o := range p.orders {
			if workClass(o) == class {
				total++
			}
		}
		return score.counts[class] == total
	}
	switch phase {
	case p.delayPhase():
		return score.delay == 0
	case 4:
		return score.used == int64(len(p.alreadyUsed))
	case 5:
		return score.distance == 0
	}
	return false
}

type nativeRouting struct {
	manager    cs.RoutingIndexManager
	model      cs.RoutingModel
	onSolution cs.GoAtSolutionCallbackWrapper
}

func (n *nativeRouting) close() {
	// Keep callbacks alive until the model no longer references them.
	if n.model != nil {
		cs.DeleteRoutingModel(n.model)
	}
	if n.onSolution != nil {
		n.onSolution.Delete()
	}
	if n.manager != nil {
		cs.DeleteRoutingIndexManager(n.manager)
	}
}
func expr(v cs.IntVar) cs.IntExpr { return v.SwigGetIntExpr() }

func (p *routingProblem) searchPhase(ctx context.Context, phase int, bound planScore, incumbent contracts.SolveResult, deadline time.Time, consider func(contracts.SolveResult)) (bool, error) {
	n := &nativeRouting{}
	defer n.close()
	count, vehicles := len(p.orders), len(p.workers)
	sink := count + vehicles
	nodes := sink + 1
	starts, ends := make([]int, vehicles), make([]int, vehicles)
	for v := range p.workers {
		starts[v] = count + v
		ends[v] = sink
	}
	n.manager = cs.NewRoutingIndexManager(nodes, vehicles, starts, ends)
	mp := cs.DefaultRoutingModelParameters()
	mp.SolverParameters.CheckSolutionPeriod = 1
	n.model = cs.NewRoutingModel(n.manager, routingModelParameters(&mp))
	r, s := n.model, n.model.Solver()
	stopped := func() bool { return ctx.Err() != nil || !time.Now().Before(deadline) }
	timeCallbacks := make([]int, vehicles)
	zero := r.RegisterUnaryTransitVector(make([]int64, nodes))
	r.SetArcCostEvaluatorOfAllVehicles(zero)
	type indexes struct{ time, cost int }
	profileIndexes := map[contracts.Transport]indexes{}
	for v, w := range p.workers {
		if stopped() {
			return false, ctx.Err()
		}
		ids, ok := profileIndexes[w.Engineer.Transport]
		if !ok {
			times := make([][]int64, nodes)
			var costs [][]int64
			if phase == 5 {
				costs = make([][]int64, nodes)
			}
			for from := 0; from < nodes; from++ {
				if stopped() {
					return false, ctx.Err()
				}
				times[from] = make([]int64, nodes)
				if phase == 5 {
					costs[from] = make([]int64, nodes)
				}
				var service int64
				var location int
				if from < count {
					service = p.orders[from].ServiceSec
					location = p.locations[p.orders[from].LocationID]
				} else if from < sink {
					location = p.workers[from-count].InitialLocation
				}
				for to := 0; to < nodes; to++ {
					if from == sink {
						continue
					}
					if to == sink {
						times[from][to] = min(service, p.horizon+1)
						continue
					}
					var target int
					if to < count {
						target = p.locations[p.orders[to].LocationID]
					} else {
						target = p.workers[to-count].InitialLocation
					}
					cell := p.input.TravelMatrix.Profiles[w.Engineer.Transport][location][target]
					if !cell.Reachable {
						times[from][to] = p.horizon + 1
						continue
					}
					times[from][to] = min(service+*cell.DurationSec, p.horizon+1)
					if phase == 5 {
						costs[from][to] = *cell.DistanceM
					}
				}
			}
			ids.time = r.RegisterTransitMatrix(times)
			ids.cost = zero
			if phase == 5 {
				ids.cost = r.RegisterTransitMatrix(costs)
			}
			profileIndexes[w.Engineer.Transport] = ids
		}
		timeCallbacks[v] = ids.time
		r.SetArcCostEvaluatorOfVehicle(ids.cost, v)
		if phase == 4 && !p.alreadyUsed[w.Engineer.ID] {
			r.SetFixedCostOfVehicle(int64(1), v)
		}
	}
	r.AddDimensionWithVehicleTransits(timeCallbacks, p.horizon, p.horizon, false, "Time")
	td := r.GetMutableDimension("Time")
	for v, w := range p.workers {
		td.CumulVar(r.Start(v)).SetValue(w.Route.StartAt.Unix() - p.origin)
		td.CumulVar(r.End(v)).SetRange(w.Route.StartAt.Unix()-p.origin, w.Engineer.Shift.End.Unix()-p.origin)
	}
	activeByClass := [3][]cs.IntVar{}
	delayVars := []cs.IntVar{}
	for i, o := range p.orders {
		if stopped() {
			return false, ctx.Err()
		}
		index := n.manager.NodeToIndex(i)
		active := r.ActiveVar(index)
		activeByClass[workClass(o)] = append(activeByClass[workClass(o)], active)
		penalty := int64(0)
		if class, ok := p.countClassForPhase(phase); ok && workClass(o) == class {
			penalty = 1
		}
		r.AddDisjunction([]int64{index}, penalty)
		allowed := []int{}
		for v, w := range p.workers {
			if w.MatchesSkills(o) && w.MatchesTransport(o) && w.HasEquipment(o) {
				start := shared.Later(shared.Later(w.Route.StartAt, o.ReceivedAt), o.Window.Start)
				if !start.After(o.Window.End) && !start.After(w.Engineer.Shift.End) && time.Duration(o.ServiceSec)*time.Second <= w.Engineer.Shift.End.Sub(start) {
					allowed = append(allowed, v)
				}
			}
		}
		lower := shared.Later(o.Window.Start, o.ReceivedAt).Unix() - p.origin
		upper := min(o.Window.End.Unix()-p.origin, p.horizon-o.ServiceSec)
		if len(allowed) == 0 || lower > upper {
			active.SetValue(0)
		} else {
			r.SetAllowedVehiclesForIndex(allowed, index)
			td.CumulVar(index).SetRange(lower, upper)
		}
		if o.WorkType == contracts.WorkTypeEmergency {
			if phase == p.delayPhase() {
				td.SetCumulVarSoftUpperBound(index, lower, 1)
			}
			if phase > p.delayPhase() {
				difference := s.MakeSum(expr(td.CumulVar(index)), -lower)
				nonnegative := s.MakeMax(difference, int64(0))
				delayVars = append(delayVars, s.MakeProd(nonnegative, expr(active)).Var())
			}
		}
	}
	// End indices are distinct per vehicle; releases must cover manager indices.
	release := make([]int64, n.manager.GetNumberOfIndices())
	for i, o := range p.orders {
		release[n.manager.NodeToIndex(i)] = o.ReceivedAt.Unix() - p.origin
	}
	for index := int64(0); index < r.Size(); index++ {
		if stopped() {
			return false, ctx.Err()
		}
		node := n.manager.IndexToNode(index)
		var service int64
		if node < count {
			service = p.orders[node].ServiceSec
		}
		departure := s.MakeSum(s.MakeSum(expr(td.CumulVar(index)), expr(td.SlackVar(index))), service)
		nextRelease := s.MakeElement(release, r.NextVar(index))
		// An inactive node loops to itself and must impose no release constraint.
		if !r.IsStart(index) {
			nextRelease = s.MakeProd(nextRelease, expr(r.ActiveVar(index)))
		}
		s.AddConstraint(s.MakeGreaterOrEqual(departure, nextRelease))
	}
	for _, kind := range []contracts.Equipment{contracts.EquipmentRouter, contracts.EquipmentTVBox} {
		demands := make([]int64, nodes)
		var total int64
		for i, o := range p.orders {
			demands[i] = o.EquipmentRequired[kind]
			total += demands[i]
		}
		if total == 0 {
			continue
		}
		capacity := make([]int64, vehicles)
		for v, w := range p.workers {
			capacity[v] = min(w.Remaining[kind], total)
		}
		callback := r.RegisterUnaryTransitVector(demands)
		r.AddDimensionWithVehicleCapacity(callback, 0, capacity, true, string(kind))
	}
	for previous := 0; previous < phase; previous++ {
		if class, ok := p.countClassForPhase(previous); ok {
			addSumEquality(s, activeByClass[class], bound.counts[class])
		}
	}
	if phase > p.delayPhase() {
		addSumUpperBound(s, delayVars, bound.delay)
	}
	if phase > 4 {
		newWorkers := []cs.IntVar{}
		for v, w := range p.workers {
			if !p.alreadyUsed[w.Engineer.ID] {
				newWorkers = append(newWorkers, r.ActiveVehicleVar(v))
			}
		}
		addSumUpperBound(s, newWorkers, bound.used-int64(len(p.alreadyUsed)))
	}
	// CP-SAT's routing translation does not carry arbitrary CP constraints.
	// Always use checked CP local search for the custom release/lex constraints.
	params := cs.DefaultRoutingSearchParameters()
	params.FirstSolutionStrategy = cs.FirstSolutionStrategy_PARALLEL_CHEAPEST_INSERTION
	params.LocalSearchMetaheuristic = cs.LocalSearchMetaheuristic_GUIDED_LOCAL_SEARCH
	params.UseFullPropagation = true
	params.UseCp = util.OptionalBoolean_BOOL_TRUE
	params.UseCpSat = util.OptionalBoolean_BOOL_FALSE
	params.UseGeneralizedCpSat = util.OptionalBoolean_BOOL_FALSE
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return false, ctx.Err()
	}
	params.TimeLimit = durationpb.New(remaining)
	params.LnsTimeLimit = durationpb.New(min(remaining, 50*time.Millisecond))
	var callbackErr error
	n.onSolution = cs.NewGoAtSolutionCallbackWrapper(func() {
		if stopped() {
			r.CancelSearch()
			return
		}
		sequences := make([][]int, vehicles)
		for v := range p.workers {
			index := r.NextVar(r.Start(v)).Value()
			for steps := 0; !r.IsEnd(index); steps++ {
				if steps >= count {
					callbackErr = computationError("OR-Tools вернул цикл в маршруте")
					r.CancelSearch()
					return
				}
				sequences[v] = append(sequences[v], n.manager.IndexToNode(index))
				index = r.NextVar(index).Value()
			}
		}
		candidate, candidateErr := p.materialize(sequences)
		if candidateErr != nil {
			callbackErr = candidateErr
			r.CancelSearch()
			return
		}
		consider(candidate)
	})
	r.AddAtSolutionCallback(n.onSolution.Wrap())
	// CancelSearch only writes atomic flags. Join before destroying the model.
	done, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		timer := time.NewTimer(max(time.Until(deadline), 0))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			r.CancelSearch()
		case <-timer.C:
			r.CancelSearch()
		case <-done:
		}
	}()
	defer func() { close(done); <-joined }()
	r.CloseModelWithParameters(routingSearchParameters(&params))
	if stopped() {
		return false, ctx.Err()
	}
	seq := p.sequences(incumbent)
	nativeSeq := make([][]int64, vehicles)
	for v, path := range seq {
		for _, i := range path {
			nativeSeq[v] = append(nativeSeq[v], n.manager.NodeToIndex(i))
		}
	}
	seed := r.ReadAssignmentFromRoutes(nativeSeq, true)
	if stopped() {
		return false, ctx.Err()
	}
	params.TimeLimit = durationpb.New(time.Until(deadline))
	var solution cs.Assignment
	if seed != nil && seed.Swigcptr() != 0 {
		solution = r.SolveFromAssignmentWithParameters(seed, routingSearchParameters(&params))
	} else {
		solution = r.SolveWithParameters(routingSearchParameters(&params))
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if callbackErr != nil {
		return false, callbackErr
	}
	if solution != nil && solution.Swigcptr() != 0 {
		sequences := make([][]int, vehicles)
		for v := range p.workers {
			index := solution.Value(r.NextVar(r.Start(v)))
			for steps := 0; !r.IsEnd(index); steps++ {
				if steps >= count {
					return false, computationError("OR-Tools вернул цикл в маршруте")
				}
				sequences[v] = append(sequences[v], n.manager.IndexToNode(index))
				index = solution.Value(r.NextVar(index))
			}
		}
		candidate, err := p.materialize(sequences)
		if err != nil {
			return false, err
		}
		consider(candidate)
	}
	if r.GetStatus() == cs.RoutingSearchStatus_ROUTING_INVALID {
		return false, computationError("OR-Tools отклонил Routing-модель")
	}
	if r.GetStatus() == cs.RoutingSearchStatus_ROUTING_INFEASIBLE {
		return false, computationError(fmt.Sprintf("OR-Tools признал этап %d несовместимым с уже проверенным допустимым планом", phase+1))
	}
	// SUCCESS only describes the incumbent/local optimum. Inspect the actual
	// search limit separately; completed is deliberately not an optimality claim.
	return !stopped() && !r.CheckLimit() && r.GetStatus() != cs.RoutingSearchStatus_ROUTING_FAIL_TIMEOUT, nil
}
func addSumEquality(s cs.Solver, vars []cs.IntVar, value int64) {
	vector := cs.NewIntVarVector()
	defer cs.DeleteIntVarVector(vector)
	for _, v := range vars {
		vector.Add(v)
	}
	s.AddConstraint(s.MakeSumEquality(vector, value))
}
func addSumUpperBound(s cs.Solver, vars []cs.IntVar, value int64) {
	vector := cs.NewIntVarVector()
	defer cs.DeleteIntVarVector(vector)
	for _, v := range vars {
		vector.Add(v)
	}
	s.AddConstraint(s.MakeSumLessOrEqual(vector, value))
}
