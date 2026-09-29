package optimized

import (
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/shared"
)

type fastOrder struct {
	location                         int
	received, windowStart, windowEnd time.Time
	service                          time.Duration
	equipment                        [2]int64
	emergency                        bool
}

type fastWorker struct {
	location   int
	start, end time.Time
	stock      [2]int64
	compatible []bool
	matrix     [][]contracts.TravelCell
}

type fastProblem struct {
	orders  []fastOrder
	workers []fastWorker
}

// Validation restricts equipment to router and tv_box. Copying two counters
// avoids allocating and looking up a stock map for every tentative insertion.
func (p *routingProblem) prepareFastEstimate() {
	f := &fastProblem{orders: make([]fastOrder, len(p.orders)), workers: make([]fastWorker, len(p.workers))}
	for i, o := range p.orders {
		f.orders[i] = fastOrder{p.locations[o.LocationID], o.ReceivedAt, o.Window.Start, o.Window.End,
			time.Duration(o.ServiceSec) * time.Second, equipmentAmounts(o.EquipmentRequired), o.WorkType == contracts.WorkTypeEmergency}
	}
	for v, w := range p.workers {
		compatible := make([]bool, len(p.orders))
		for i, o := range p.orders {
			compatible[i] = w.MatchesSkills(o) && w.MatchesTransport(o)
		}
		f.workers[v] = fastWorker{w.Location, w.Time, w.Engineer.Shift.End, equipmentAmounts(w.Remaining), compatible,
			p.input.TravelMatrix.Profiles[w.Engineer.Transport]}
	}
	p.fast = f
}

// Keep time.Time arithmetic identical to AppendCandidate, including fractional
// timestamps, release-before-departure, asymmetric travel and shift bounds.
func (p *routingProblem) fastEstimate(v int, seq []int) routeEstimate {
	w := &p.fast.workers[v]
	stock, location, now := w.stock, w.location, w.start
	result := routeEstimate{finish: now, ok: true}
	for _, i := range seq {
		o := &p.fast.orders[i]
		if !w.compatible[i] || stock[0] < o.equipment[0] || stock[1] < o.equipment[1] {
			return routeEstimate{}
		}
		cell := w.matrix[location][o.location]
		if !cell.Reachable {
			return routeEstimate{}
		}
		departure := shared.Later(now, o.received).UTC()
		travel := time.Duration(*cell.DurationSec) * time.Second
		if departure.After(w.end) || travel > w.end.Sub(departure) {
			return routeEstimate{}
		}
		start := shared.Later(departure.Add(travel), o.windowStart).UTC()
		if start.After(o.windowEnd) || start.After(w.end) || o.service > w.end.Sub(start) {
			return routeEstimate{}
		}
		now, location = start.Add(o.service), o.location
		stock[0] -= o.equipment[0]
		stock[1] -= o.equipment[1]
		result.distance += *cell.DistanceM
		if o.emergency {
			result.delay += int64(start.Sub(shared.Later(o.windowStart, o.received)) / time.Second)
		}
	}
	result.finish = now
	return result
}
