package planner

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/shared"
)

// Inserter exhaustively checks gaps for one new ordinary order. It never
// retimes a fixed visit or redirects a protected trip. The zero value is usable.
type Inserter struct{ now func() time.Time }

var _ contracts.Planner = (*Inserter)(nil)

func NewInserter() *Inserter { return &Inserter{} }

type insertionInput struct {
	orders     map[string]contracts.Order
	states     map[string]contracts.EngineerState
	engineers  map[string]contracts.Engineer
	locations  map[string]int
	protected  map[string]bool
	routeIndex map[string]int
	remaining  map[string]map[contracts.Equipment]int64
	legIDs     map[string]bool
	fresh      contracts.Order
}

func (i *Inserter) Solve(ctx context.Context, in contracts.SolveRequest) (contracts.SolveResult, error) {
	now := i.now
	if now == nil {
		now = time.Now
	}
	started := now()
	if err := ctx.Err(); err != nil {
		return contracts.SolveResult{}, err
	}
	if in.Mode != contracts.SolveModeInsertOnly {
		return contracts.SolveResult{}, shared.InvalidInput("mode", "Inserter поддерживает только insert_only")
	}
	if err := shared.ValidateInput(ctx, in); err != nil {
		return contracts.SolveResult{}, err
	}
	data, err := prepareInsertion(ctx, in)
	if err != nil {
		return contracts.SolveResult{}, err
	}
	deadline := started.Add(time.Duration(in.TimeLimitMS) * time.Millisecond)
	stop := func() (bool, error) {
		if err := ctx.Err(); err != nil {
			return true, err
		}
		return !now().Before(deadline), nil
	}
	fallback := func(timedOut bool) contracts.SolveResult {
		result := contracts.SolveResult{Routes: cloneRoutes(in.FixedRoutes), Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}
		code, message := contracts.ReasonNoFeasibleInsertion, "Все зазоры проверены: допустимой вставки нет."
		if timedOut {
			result.Termination = contracts.TerminationTimeLimit
			code, message = contracts.ReasonNotAssignedBySolver, "Поиск остановлен по лимиту времени; старый план сохранён."
		}
		result.Unassigned = append(result.Unassigned, shared.Unassigned(data.fresh, code, message))
		return result
	}
	engineers := append([]contracts.Engineer(nil), in.Engineers...)
	sort.Slice(engineers, func(i, j int) bool {
		if engineers[i].SourceOrder != engineers[j].SourceOrder {
			return engineers[i].SourceOrder < engineers[j].SourceOrder
		}
		return engineers[i].ID < engineers[j].ID
	})
	for _, engineer := range engineers {
		if done, err := stop(); err != nil {
			return contracts.SolveResult{}, err
		} else if done {
			return fallback(true), nil
		}
		state := data.states[engineer.ID]
		if !matchesOrder(engineer, data.fresh) {
			continue
		}
		enough := true
		for kind, count := range data.fresh.EquipmentRequired {
			if data.remaining[engineer.ID][kind] < count {
				enough = false
				break
			}
		}
		if !enough {
			continue
		}
		index, exists := data.routeIndex[engineer.ID]
		route := contracts.Route{EngineerID: engineer.ID, StartLocationID: state.StartLocationID, StartAt: shared.Later(state.AvailableFrom, engineer.Shift.Start).UTC(), Visits: []contracts.Visit{}, Legs: []contracts.Leg{}}
		if exists {
			route = in.FixedRoutes[index]
		}
		for position := 0; position <= len(route.Visits); position++ {
			if done, err := stop(); err != nil {
				return contracts.SolveResult{}, err
			} else if done {
				return fallback(true), nil
			}
			if position == 0 && len(route.Legs) > 0 && data.protected[route.Legs[0].ID] {
				continue
			}
			candidate, ok := data.insertAt(route, position, engineer, in.TravelMatrix)
			if !ok {
				continue
			}
			if done, err := stop(); err != nil {
				return contracts.SolveResult{}, err
			} else if done {
				return fallback(true), nil
			}
			// Allocate collision-free identifiers only after a feasible gap is found.
			number := 0
			for j := range candidate.Legs {
				if candidate.Legs[j].ID == "" {
					for {
						number++
						id := fmt.Sprintf("insert-leg-%d", number)
						if !data.legIDs[id] {
							candidate.Legs[j].ID = id
							data.legIDs[id] = true
							break
						}
					}
				}
			}
			result := contracts.SolveResult{Routes: cloneRoutes(in.FixedRoutes), Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}
			if exists {
				result.Routes[index] = candidate
			} else {
				result.Routes = append(result.Routes, candidate)
			}
			return result, nil
		}
	}
	if done, err := stop(); err != nil {
		return contracts.SolveResult{}, err
	} else if done {
		return fallback(true), nil
	}
	return fallback(false), nil
}

func prepareInsertion(ctx context.Context, in contracts.SolveRequest) (*insertionInput, error) {
	d := &insertionInput{orders: map[string]contracts.Order{}, states: map[string]contracts.EngineerState{}, engineers: map[string]contracts.Engineer{}, locations: map[string]int{}, protected: map[string]bool{}, routeIndex: map[string]int{}, remaining: map[string]map[contracts.Equipment]int64{}, legIDs: map[string]bool{}}
	for _, o := range in.Orders {
		d.orders[o.ID] = o
	}
	for _, s := range in.EngineerStates {
		d.states[s.EngineerID] = s
		stock := map[contracts.Equipment]int64{}
		for k, n := range s.EquipmentAvailable {
			stock[k] = n
		}
		d.remaining[s.EngineerID] = stock
	}
	for _, e := range in.Engineers {
		d.engineers[e.ID] = e
	}
	for i, id := range in.TravelMatrix.LocationIDs {
		d.locations[id] = i
	}
	for _, id := range in.ProtectedLegIDs {
		if id == "" || d.protected[id] {
			return nil, shared.InvalidInput("protected_leg_ids", "защищённые ID должны быть непустыми и уникальными")
		}
		d.protected[id] = true
	}
	used := map[string]bool{}
	protectedSeen := map[string]bool{}
	for ri, r := range in.FixedRoutes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		field := fmt.Sprintf("fixed_routes[%d]", ri)
		e, ok := d.engineers[r.EngineerID]
		if !ok {
			return nil, shared.InvalidInput(field+".engineer_id", "неизвестный инженер")
		}
		if _, ok := d.routeIndex[e.ID]; ok {
			return nil, shared.InvalidInput(field+".engineer_id", "для инженера допускается один маршрут")
		}
		d.routeIndex[e.ID] = ri
		state := d.states[e.ID]
		if err := shared.ValidateTime(field+".start_at", r.StartAt); err != nil {
			return nil, err
		}
		if r.StartLocationID != state.StartLocationID || !r.StartAt.Equal(shared.Later(state.AvailableFrom, e.Shift.Start)) {
			return nil, shared.InvalidInput(field, "начало маршрута не соответствует состоянию инженера и смене")
		}
		if len(r.Visits) != len(r.Legs) {
			return nil, shared.InvalidInput(field, "число legs должно совпадать с числом visits")
		}
		previousLocation, previousEnd := r.StartLocationID, r.StartAt
		for j, v := range r.Visits {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			vf := fmt.Sprintf("%s.visits[%d]", field, j)
			o, ok := d.orders[v.OrderID]
			if !ok || used[v.OrderID] {
				return nil, shared.InvalidInput(vf+".order_id", "неизвестная или повторная заявка")
			}
			used[v.OrderID] = true
			leg := r.Legs[j]
			for name, value := range map[string]time.Time{"arrival_at": v.ArrivalAt, "start_at": v.StartAt, "end_at": v.EndAt, "leg.start_at": leg.StartAt, "leg.end_at": leg.EndAt} {
				if err := shared.ValidateTime(vf+"."+name, value); err != nil {
					return nil, err
				}
			}
			if leg.ID == "" || d.legIDs[leg.ID] {
				return nil, shared.InvalidInput(vf, "ID дорожных участков должны быть непустыми и уникальными")
			}
			d.legIDs[leg.ID] = true
			if !matchesOrder(e, o) {
				return nil, shared.InvalidInput(vf, "старое назначение нарушает навыки или транспорт")
			}
			if o.Execution != nil && o.Execution.EngineerID != e.ID {
				return nil, shared.InvalidInput(vf, "фиксированный исполнитель не совпадает с execution")
			}
			if v.ArrivalAt.After(v.StartAt) || v.StartAt.Before(o.ReceivedAt) || v.StartAt.Before(o.Window.Start) || v.StartAt.After(o.Window.End) || v.StartAt.Before(previousEnd) || v.StartAt.Before(e.Shift.Start) || v.EndAt.After(e.Shift.End) || !v.StartAt.Add(time.Duration(o.ServiceSec)*time.Second).Equal(v.EndAt) {
				return nil, shared.InvalidInput(vf, "старый визит нарушает окна, длительность, смену или доступность")
			}
			if leg.ToLocationID != o.LocationID || leg.FromLocationID == "" || leg.StartAt.Before(o.ReceivedAt) || leg.StartAt.After(leg.EndAt) || !leg.EndAt.Equal(v.ArrivalAt) || leg.DistanceM < 0 || leg.GeoContextID == "" {
				return nil, shared.InvalidInput(vf, "дорожный участок не соответствует старому визиту")
			}
			if d.protected[leg.ID] {
				if j != 0 {
					return nil, shared.InvalidInput("protected_leg_ids", "защищать можно только первый участок маршрута")
				}
				protectedSeen[leg.ID] = true
				// The full saved leg may start at a previous location/time. State refers
				// to the current point along it. Its remaining travel is the saved end,
				// never a lookup in the freshly calculated matrix.
				if leg.StartAt.After(r.StartAt) && leg.FromLocationID != r.StartLocationID {
					return nil, shared.InvalidInput(vf, "будущая защищённая поездка начинается не из текущей точки")
				}
			} else {
				if leg.FromLocationID != previousLocation || leg.StartAt.Before(previousEnd) {
					return nil, shared.InvalidInput(vf, "дорожный участок нарушает последовательность маршрута")
				}
				cell := in.TravelMatrix.Profiles[e.Transport][d.locations[previousLocation]][d.locations[o.LocationID]]
				if !cell.Reachable || !leg.StartAt.Add(time.Duration(*cell.DurationSec)*time.Second).Equal(leg.EndAt) || leg.DistanceM != *cell.DistanceM {
					return nil, shared.InvalidInput(vf, "незащищённый дорожный участок не соответствует travel_matrix")
				}
			}
			for kind, count := range o.EquipmentRequired {
				if d.remaining[e.ID][kind] < count {
					return nil, shared.InvalidInput(vf, "оборудование фиксированных визитов превышает остаток")
				}
				d.remaining[e.ID][kind] -= count
			}
			previousLocation, previousEnd = o.LocationID, v.EndAt
		}
	}
	for id := range d.protected {
		if !protectedSeen[id] {
			return nil, shared.InvalidInput("protected_leg_ids", "защищённый участок не найден")
		}
	}
	freshCount := 0
	for _, o := range in.Orders {
		if !used[o.ID] {
			d.fresh = o
			freshCount++
		}
	}
	if freshCount != 1 {
		return nil, shared.InvalidInput("orders", "insert_only требует старые визиты и ровно одну новую заявку")
	}
	if d.fresh.WorkType == contracts.WorkTypeEmergency || d.fresh.Status != contracts.OrderStatusActive {
		return nil, shared.InvalidInput("orders", "новая заявка должна быть обычной и иметь статус active")
	}
	return d, nil
}

func (d *insertionInput) insertAt(r contracts.Route, pos int, e contracts.Engineer, m contracts.TravelMatrix) (contracts.Route, bool) {
	from, ready := r.StartLocationID, r.StartAt
	if pos > 0 {
		v := r.Visits[pos-1]
		from, ready = d.orders[v.OrderID].LocationID, v.EndAt
	}
	o := d.fresh
	cell := m.Profiles[e.Transport][d.locations[from]][d.locations[o.LocationID]]
	w := shared.Worker{Engineer: e, Time: ready}
	visit, first, ok := w.AppendCandidate(o, cell)
	if !ok {
		return contracts.Route{}, false
	}
	first.FromLocationID = from
	first.GeoContextID = m.GeoContextID
	var second contracts.Leg
	if pos < len(r.Visits) {
		next := r.Visits[pos]
		old := d.orders[next.OrderID]
		travel := m.Profiles[e.Transport][d.locations[o.LocationID]][d.locations[old.LocationID]]
		if !travel.Reachable {
			return contracts.Route{}, false
		}
		departure := next.ArrivalAt.Add(-time.Duration(*travel.DurationSec) * time.Second)
		if departure.Before(visit.EndAt) || departure.Before(old.ReceivedAt) {
			return contracts.Route{}, false
		}
		second = contracts.Leg{FromLocationID: o.LocationID, ToLocationID: old.LocationID, StartAt: departure, EndAt: next.ArrivalAt, DistanceM: *travel.DistanceM, GeoContextID: m.GeoContextID}
	}
	candidate := cloneRoute(r)
	candidate.Visits = append(candidate.Visits, contracts.Visit{})
	copy(candidate.Visits[pos+1:], candidate.Visits[pos:])
	candidate.Visits[pos] = visit
	if pos == len(r.Visits) {
		candidate.Legs = append(candidate.Legs, first)
	} else {
		candidate.Legs = append(candidate.Legs, contracts.Leg{})
		copy(candidate.Legs[pos+2:], candidate.Legs[pos+1:])
		candidate.Legs[pos], candidate.Legs[pos+1] = first, second
	}
	return candidate, true
}

func matchesOrder(e contracts.Engineer, o contracts.Order) bool {
	if o.RequiredTransport != nil && *o.RequiredTransport != e.Transport {
		return false
	}
	skills := map[string]bool{}
	for _, s := range e.Skills {
		skills[s] = true
	}
	for _, s := range o.RequiredSkills {
		if !skills[s] {
			return false
		}
	}
	return true
}
func cloneRoute(r contracts.Route) contracts.Route {
	if r.Visits != nil {
		r.Visits = append([]contracts.Visit{}, r.Visits...)
	}
	if r.Legs != nil {
		r.Legs = append([]contracts.Leg{}, r.Legs...)
		for i := range r.Legs {
			if r.Legs[i].Geometry != nil {
				r.Legs[i].Geometry = append([]contracts.Point{}, r.Legs[i].Geometry...)
			}
		}
	}
	return r
}
func cloneRoutes(routes []contracts.Route) []contracts.Route {
	if routes == nil {
		return nil
	}
	out := make([]contracts.Route, len(routes))
	for i, r := range routes {
		out[i] = cloneRoute(r)
	}
	return out
}
