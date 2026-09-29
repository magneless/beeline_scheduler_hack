package optimized

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

type searchPoolEntry struct {
	seq   [][]int
	score planScore
}

func sameSequences(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for v := range a {
		if len(a[v]) != len(b[v]) {
			return false
		}
		for i := range a[v] {
			if a[v][i] != b[v][i] {
				return false
			}
		}
	}
	return true
}

func (p *routingProblem) geographicRemoval(seq [][]int, rng *rand.Rand) {
	if len(p.orders) == 0 {
		return
	}
	pivot := rng.Intn(len(p.orders))
	near := make([]int, len(p.orders))
	distance := make([]int64, len(p.orders))
	for i := range near {
		near[i] = i
		distance[i] = 1 << 62
		from, to := p.locations[p.orders[pivot].LocationID], p.locations[p.orders[i].LocationID]
		for _, profile := range p.input.TravelMatrix.Profiles {
			cell := profile[from][to]
			if cell.Reachable {
				distance[i] = min(distance[i], *cell.DistanceM)
			}
		}
	}
	sort.SliceStable(near, func(i, j int) bool { return distance[near[i]] < distance[near[j]] })
	removed := make([]bool, len(p.orders))
	for _, i := range near[:min(len(near), max(3, len(near)/5))] {
		removed[i] = true
	}
	for v, route := range seq {
		kept := route[:0]
		for _, i := range route {
			if !removed[i] {
				kept = append(kept, i)
			}
		}
		seq[v] = kept
	}
}

func (p *routingProblem) enhancedSearch(ctx context.Context, best contracts.SolveResult, deadline time.Time, rng *rand.Rand) (contracts.SolveResult, error) {
	kinds := []string{"random", "route", "geographic", "fresh"}
	if p.search.CrewElimination {
		kinds = append(kinds, "crew")
	}
	if p.search.Segments {
		kinds = append(kinds, "chains", "exchange")
	}
	weights := make([]float64, len(kinds))
	for i := range weights {
		weights[i] = 1
	}
	score := p.score(best)
	current := p.sequences(best)
	pool := []searchPoolEntry{{copySequences(current), score}}
	crewCursor := 0
	for iteration := 0; !searchStopped(ctx, deadline); iteration++ {
		if p.diagnostics != nil {
			p.diagnostics.Iterations++
		}
		index := iteration % len(kinds)
		if p.search.Adaptive && iteration >= len(kinds) {
			total := 0.0
			for _, w := range weights {
				total += w
			}
			choice := rng.Float64() * total
			for i, w := range weights {
				index = i
				choice -= w
				if choice <= 0 {
					break
				}
			}
			current = copySequences(pool[rng.Intn(len(pool))].seq)
		}
		kind := kinds[index]
		started := time.Now()
		// Reserve comparable bounded attempts so no single failed reconstruction
		// consumes the whole remaining budget and starves other operators.
		attemptEnd := started.Add(min(250*time.Millisecond, time.Until(deadline)))
		candidate := best
		switch kind {
		case "crew":
			candidate = p.eliminateCrew(ctx, best, attemptEnd, rng, crewCursor)
			crewCursor++
		case "chains", "exchange":
			candidate = p.improveSegments(ctx, best, attemptEnd, kind == "exchange")
		default:
			seq := copySequences(current)
			switch kind {
			case "fresh":
				seq = make([][]int, len(p.workers))
			case "geographic":
				p.geographicRemoval(seq, rng)
			default:
				removedRoute := rng.Intn(len(seq))
				fraction := 0.15 + rng.Float64()*0.3
				for v, route := range seq {
					kept := route[:0]
					for _, i := range route {
						if (kind != "route" || v != removedRoute) && rng.Float64() > fraction {
							kept = append(kept, i)
						}
					}
					seq[v] = kept
				}
			}
			seq = p.feasiblePrefixes(seq)
			seq = p.repair(ctx, seq, rng, attemptEnd, iteration%6)
			var err error
			candidate, err = p.materialize(seq)
			if err != nil {
				return best, err
			}
		}
		candidateScore := p.score(candidate)
		improved := betterPlan(candidateScore, score, p.input.EmergencyFirst)
		if improved {
			best, score = candidate, candidateScore
			if p.diagnostics != nil {
				p.diagnostics.Improvements++
			}
		}
		seq := p.sequences(candidate)
		if improved || iteration%7 == 0 {
			current = seq
		} else {
			current = p.sequences(best)
		}
		if p.search.Adaptive {
			duplicate := false
			for _, entry := range pool {
				if sameSequences(entry.seq, seq) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				pool = append(pool, searchPoolEntry{copySequences(seq), candidateScore})
				sort.SliceStable(pool, func(i, j int) bool { return betterPlan(pool[i].score, pool[j].score, p.input.EmergencyFirst) })
				if len(pool) > 4 {
					pool = pool[:4]
				}
			}
			reward := 0.2
			if improved {
				reward = min(8.0, 200/float64(max(1, time.Since(started).Milliseconds())))
			}
			weights[index] = 0.85*weights[index] + 0.15*reward
		}
		if p.diagnostics != nil {
			stat := p.diagnostics.Operators[kind]
			stat.Attempts++
			stat.ElapsedMS += time.Since(started).Milliseconds()
			if improved {
				stat.Improvements++
			}
			p.diagnostics.Operators[kind] = stat
		}
	}
	return best, ctx.Err()
}
