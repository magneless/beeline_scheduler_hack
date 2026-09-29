package optimized

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

type routeEstimate struct {
	finish          time.Time
	distance, delay int64
	ok              bool
}

func (p *routingProblem) estimate(v int, seq []int) routeEstimate {
	if p.diagnostics != nil {
		p.diagnostics.Estimates++
	}
	if p.fast != nil {
		return p.fastEstimate(v, seq)
	}
	return p.referenceEstimate(v, seq)
}

func (p *routingProblem) referenceEstimate(v int, seq []int) routeEstimate {
	w := p.workers[v]
	w.Remaining = cloneStock(w.Remaining)
	result := routeEstimate{finish: w.Time, ok: true}
	for _, i := range seq {
		o := p.orders[i]
		if !w.MatchesSkills(o) || !w.MatchesTransport(o) || !w.HasEquipment(o) {
			return routeEstimate{}
		}
		target := p.locations[o.LocationID]
		visit, leg, ok := w.AppendCandidate(o, p.input.TravelMatrix.Profiles[w.Engineer.Transport][w.Location][target])
		if !ok {
			return routeEstimate{}
		}
		w.Location = target
		w.Time = visit.EndAt
		result.finish = w.Time
		result.distance += leg.DistanceM
		if o.WorkType == contracts.WorkTypeEmergency {
			release := o.Window.Start
			if o.ReceivedAt.After(release) {
				release = o.ReceivedAt
			}
			result.delay += int64(visit.StartAt.Sub(release) / time.Second)
		}
		for k, n := range o.EquipmentRequired {
			w.Remaining[k] -= n
		}
	}
	return result
}
func cloneStock(stock map[contracts.Equipment]int64) map[contracts.Equipment]int64 {
	copy := make(map[contracts.Equipment]int64, len(stock))
	for k, n := range stock {
		copy[k] = n
	}
	return copy
}
func copySequences(s [][]int) [][]int {
	r := make([][]int, len(s))
	for i := range s {
		r[i] = append([]int(nil), s[i]...)
	}
	return r
}
func insertIndex(s []int, pos, i int) []int {
	r := make([]int, len(s)+1)
	copy(r, s[:pos])
	r[pos] = i
	copy(r[pos+1:], s[pos:])
	return r
}

type cachedInsertion struct {
	generation int
	best       repairInsertion
	second     float64
}

type repairInsertion struct {
	order, vehicle, position int
	cost, regret             float64
	found                    bool
}

func (p *routingProblem) repair(ctx context.Context, seq [][]int, rng *rand.Rand, deadline time.Time, variant int) [][]int {
	return p.repairRestricted(ctx, seq, rng, deadline, variant, -1)
}

func (p *routingProblem) repairRestricted(ctx context.Context, seq [][]int, rng *rand.Rand, deadline time.Time, variant, forbidden int) [][]int {
	assigned := make([]bool, len(p.orders))
	for _, r := range seq {
		for _, i := range r {
			assigned[i] = true
		}
	}
	noise := make([]float64, len(p.orders))
	for i := range noise {
		noise[i] = 0.7 + rng.Float64()*0.6
	}
	versions := make([]int, len(seq))
	for v := range versions {
		versions[v] = 1
	}
	// An insertion only invalidates positions in the modified crew's route.
	cache := make([][]cachedInsertion, len(p.orders))
	for i := range cache {
		cache[i] = make([]cachedInsertion, len(seq))
	}
	for {
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			return seq
		}
		estimates := make([]routeEstimate, len(seq))
		for v, r := range seq {
			estimates[v] = p.estimate(v, r)
		}
		chosen := repairInsertion{}
		bestClass := 4
		for i, o := range p.orders {
			if assigned[i] || workClass(o) > bestClass {
				continue
			}
			if ctx.Err() != nil || !time.Now().Before(deadline) {
				return seq
			}
			ins := repairInsertion{order: i, cost: math.Inf(1)}
			second := math.Inf(1)
			for v, r := range seq {
				if v == forbidden {
					continue
				}
				entry := &cache[i][v]
				if entry.generation != versions[v] {
					entry.generation = versions[v]
					entry.best = repairInsertion{order: i, vehicle: v, cost: math.Inf(1)}
					entry.second = math.Inf(1)
					for pos := 0; pos <= len(r); pos++ {
						if pos%16 == 0 && (ctx.Err() != nil || !time.Now().Before(deadline)) {
							return seq
						}
						trial := p.estimate(v, insertIndex(r, pos, i))
						if !trial.ok {
							continue
						}
						// Construction ranking only; acceptance uses betterPlan.
						cost := float64(trial.finish.Sub(estimates[v].finish)/time.Second) + float64(trial.distance-estimates[v].distance)/10
						if len(r) == 0 && !p.alreadyUsed[p.workers[v].Engineer.ID] {
							cost += 1200
						}
						if o.WorkType == contracts.WorkTypeEmergency {
							cost += float64(trial.delay-estimates[v].delay) * 10
						}
						if variant > 0 {
							cost *= noise[i]
						}
						if cost < entry.best.cost {
							entry.second = entry.best.cost
							entry.best.cost = cost
							entry.best.position = pos
							entry.best.found = true
						} else if cost < entry.second {
							entry.second = cost
						}
					}
				}
				if !entry.best.found {
					continue
				}
				if entry.best.cost < ins.cost {
					second = min(ins.cost, entry.second)
					ins = entry.best
				} else {
					second = min(second, entry.best.cost)
				}
			}
			if !ins.found {
				continue
			}
			ins.regret = second - ins.cost
			if !chosen.found || workClass(o) < bestClass || (variant%3 != 2 && ins.regret > chosen.regret) || (variant%3 == 2 && ins.cost < chosen.cost) || (ins.regret == chosen.regret && ins.cost < chosen.cost) {
				chosen = ins
				bestClass = workClass(o)
			}
		}
		if !chosen.found {
			return seq
		}
		assigned[chosen.order] = true
		seq[chosen.vehicle] = insertIndex(seq[chosen.vehicle], chosen.position, chosen.order)
		versions[chosen.vehicle]++
	}
}

// repairSearch explores regret-2/cheapest insertions and partial route rebuilds.
// All trials use the same hard constraints as materialize. Heuristic costs only
// order candidates; the incumbent changes strictly by the public lexicographic
// objective. A timeout therefore retains a valid plan at least as good as the
// input plan. The caller reserves time for unassigned-order explanations.
func (p *routingProblem) repairSearch(ctx context.Context, best contracts.SolveResult, deadline time.Time) (contracts.SolveResult, error) {
	if len(p.workers) == 0 || len(p.orders) == 0 {
		return best, ctx.Err()
	}
	seed := p.search.Seed
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))
	if p.search.CrewElimination || p.search.Segments || p.search.Adaptive {
		if p.search.RefineAfterRepair {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return best, ctx.Err()
			}
			var err error
			best, err = p.legacyRepairSearch(ctx, best, deadline.Add(-remaining*3/10), rng)
			if err != nil {
				return best, err
			}
		}
		return p.enhancedSearch(ctx, best, deadline, rng)
	}
	return p.legacyRepairSearch(ctx, best, deadline, rng)
}

func (p *routingProblem) legacyRepairSearch(ctx context.Context, best contracts.SolveResult, deadline time.Time, rng *rand.Rand) (contracts.SolveResult, error) {
	score := p.score(best)
	current := p.sequences(best)
	for iteration := 0; time.Now().Before(deadline); iteration++ {
		if p.diagnostics != nil {
			p.diagnostics.Iterations++
		}
		if err := ctx.Err(); err != nil {
			return best, err
		}
		seq := copySequences(current)
		if iteration < 3 || iteration%16 == 0 {
			seq = make([][]int, len(p.workers))
		} else {
			// Mix a removed route with random removals to explore both crew count and coverage.
			route := rng.Intn(len(seq))
			fraction := 0.15 + rng.Float64()*0.3
			for v, r := range seq {
				kept := r[:0]
				for _, i := range r {
					if (iteration%3 != 0 || v != route) && rng.Float64() > fraction {
						kept = append(kept, i)
					}
				}
				seq[v] = kept
			}
		}
		seq = p.feasiblePrefixes(seq)
		seq = p.repair(ctx, seq, rng, deadline, iteration%6)
		candidate, err := p.materialize(seq)
		if err != nil {
			return best, err
		}
		candidateScore := p.score(candidate)
		if betterPlan(candidateScore, score, p.input.EmergencyFirst) {
			if p.diagnostics != nil {
				p.diagnostics.Improvements++
			}
			best, score = candidate, candidateScore
			current = seq
		} else if iteration%7 == 0 {
			current = seq
		} else {
			current = p.sequences(best)
		}
	}
	return best, ctx.Err()
}

// Removing a visit is not necessarily feasible on an asymmetric road matrix.
// Keep only feasible prefixes; omitted orders are eligible for repair again.
func (p *routingProblem) feasiblePrefixes(seq [][]int) [][]int {
	for v, route := range seq {
		kept := make([]int, 0, len(route))
		for _, order := range route {
			trial := append(kept, order)
			if p.estimate(v, trial).ok {
				kept = trial
			}
		}
		seq[v] = kept
	}
	return seq
}
