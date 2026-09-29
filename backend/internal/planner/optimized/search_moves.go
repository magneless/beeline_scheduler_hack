package optimized

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func searchStopped(ctx context.Context, deadline time.Time) bool {
	return ctx.Err() != nil || !time.Now().Before(deadline)
}

// An excluded crew cannot receive jobs again during this attempt. Already-used
// crews remain counted by the domain objective even if their remaining route is
// empty, so excluding them cannot improve the crew-count objective.
func (p *routingProblem) eliminateCrew(ctx context.Context, best contracts.SolveResult, deadline time.Time, rng *rand.Rand, cursor int) contracts.SolveResult {
	initial := p.sequences(best)
	crews := []int{}
	for v, seq := range initial {
		if len(seq) > 0 && !p.alreadyUsed[p.workers[v].Engineer.ID] {
			crews = append(crews, v)
		}
	}
	sort.SliceStable(crews, func(i, j int) bool { return len(initial[crews[i]]) < len(initial[crews[j]]) })
	if len(crews) == 0 {
		return best
	}
	forbidden := crews[cursor%len(crews)]
	bestScore := p.score(best)
	for variant := 0; variant < 3 && !searchStopped(ctx, deadline); variant++ {
		seq := copySequences(initial)
		seq[forbidden] = nil
		remaining := time.Until(deadline)
		seq = p.repairRestricted(ctx, seq, rng, time.Now().Add(remaining/time.Duration(3-variant)), variant, forbidden)
		candidate, err := p.materialize(seq)
		if err == nil && betterPlan(p.score(candidate), bestScore, p.input.EmergencyFirst) {
			best, bestScore = candidate, p.score(candidate)
		}
	}
	return best
}

// Only the changed routes need estimation. An apparent improvement still passes
// through materialize and the exact acceptance rule before it replaces best.
func (p *routingProblem) acceptChangedRoutes(best contracts.SolveResult, seq [][]int, estimates []routeEstimate, score planScore, a, b int, nextA, nextB []int) (contracts.SolveResult, bool) {
	x := p.estimate(a, nextA)
	if !x.ok {
		return best, false
	}
	trial := score
	trial.delay += x.delay - estimates[a].delay
	trial.distance += x.distance - estimates[a].distance
	if !p.alreadyUsed[p.workers[a].Engineer.ID] {
		if len(seq[a]) > 0 && len(nextA) == 0 {
			trial.used--
		}
		if len(seq[a]) == 0 && len(nextA) > 0 {
			trial.used++
		}
	}
	if b != a {
		y := p.estimate(b, nextB)
		if !y.ok {
			return best, false
		}
		trial.delay += y.delay - estimates[b].delay
		trial.distance += y.distance - estimates[b].distance
		if !p.alreadyUsed[p.workers[b].Engineer.ID] {
			if len(seq[b]) > 0 && len(nextB) == 0 {
				trial.used--
			}
			if len(seq[b]) == 0 && len(nextB) > 0 {
				trial.used++
			}
		}
	}
	// Estimates round durations to seconds; the exact public score subtracts Unix
	// timestamps. For fractional timestamps defer close calls to the full score.
	uncertainty := int64(len(seq[a]) + len(nextA))
	if b != a {
		uncertainty += int64(len(seq[b]) + len(nextB))
	}
	if trial.delay > score.delay+uncertainty {
		return best, false
	}
	if trial.delay == score.delay && (trial.used > score.used || trial.used == score.used && trial.distance >= score.distance) {
		return best, false
	}
	candidateSeq := append([][]int(nil), seq...)
	candidateSeq[a] = nextA
	if b != a {
		candidateSeq[b] = nextB
	}
	candidate, err := p.materialize(candidateSeq)
	if err != nil || !betterPlan(p.score(candidate), score, p.input.EmergencyFirst) {
		return best, false
	}
	return candidate, true
}

func spliceSegment(seq []int, begin, count int, replacement []int) []int {
	result := make([]int, 0, len(seq)-count+len(replacement))
	result = append(result, seq[:begin]...)
	result = append(result, replacement...)
	return append(result, seq[begin+count:]...)
}

// Move chains of up to three visits, including reordering within one route, or
// exchange segments of one or two visits. Never reverse a segment implicitly:
// road costs are directed and the original order can carry release constraints.
func (p *routingProblem) improveSegments(ctx context.Context, best contracts.SolveResult, deadline time.Time, exchange bool) contracts.SolveResult {
	for !searchStopped(ctx, deadline) {
		seq, score := p.sequences(best), p.score(best)
		estimates := make([]routeEstimate, len(seq))
		order := make([]int, len(seq))
		for v, route := range seq {
			estimates[v], order[v] = p.estimate(v, route), v
		}
		sort.SliceStable(order, func(i, j int) bool { return estimates[order[i]].distance > estimates[order[j]].distance })
		improved := false
	search:
		for _, a := range order {
			for length := min(3, len(seq[a])); length > 0; length-- {
				if exchange && length > 2 {
					continue
				}
				for start := 0; start+length <= len(seq[a]); start++ {
					chain := seq[a][start : start+length]
					without := spliceSegment(seq[a], start, length, nil)
					for _, b := range order {
						if exchange && b <= a {
							continue
						}
						destination := seq[b]
						if a == b {
							destination = without
						}
						maxOther := 0
						if exchange {
							maxOther = min(2, len(destination))
						}
						for other := 0; other <= maxOther; other++ {
							if exchange && other == 0 {
								continue
							}
							for pos := 0; pos+other <= len(destination); pos++ {
								if searchStopped(ctx, deadline) {
									return best
								}
								if a == b && pos == start {
									continue
								}
								nextA := without
								if exchange {
									nextA = spliceSegment(seq[a], start, length, destination[pos:pos+other])
								}
								nextB := spliceSegment(destination, pos, other, chain)
								if a == b {
									nextA = nextB
								}
								if candidate, ok := p.acceptChangedRoutes(best, seq, estimates, score, a, b, nextA, nextB); ok {
									best, improved = candidate, true
									break search
								}
							}
						}
					}
				}
			}
		}
		if !improved {
			break
		}
	}
	return best
}
