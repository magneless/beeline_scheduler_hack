package optimized

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/shared"
)

// Optimized uses OR-Tools Routing when compiled with the ortools build tag.
// Every call owns a native model; input slices and maps are never modified.
type Optimized struct{}

var _ contracts.Planner = (*Optimized)(nil)

// New returns an OR-Tools planner.
func New() *Optimized { return &Optimized{} }

func (*Optimized) Solve(ctx context.Context, input contracts.SolveRequest) (contracts.SolveResult, error) {
	started := time.Now()
	if err := ctx.Err(); err != nil {
		return contracts.SolveResult{}, err
	}
	if input.Mode != contracts.SolveModeOptimized {
		return contracts.SolveResult{}, shared.InvalidInput("mode", "Optimized поддерживает только optimized")
	}
	if err := shared.ValidateInput(ctx, input); err != nil {
		return contracts.SolveResult{}, err
	}
	return solveRouting(ctx, input, started.Add(time.Duration(input.TimeLimitMS)*time.Millisecond))
}

func computationError(message string) error {
	return &contracts.ContractError{Code: "COMPUTATION_FAILED", Message: message}
}

type routingProblem struct {
	input       contracts.SolveRequest
	orders      []contracts.Order
	workers     []shared.Worker
	locations   map[string]int
	orderIndex  map[string]int
	workerIndex map[string]int
	alreadyUsed map[string]bool
	origin      int64
	horizon     int64
}

func prepareRouting(input contracts.SolveRequest) (*routingProblem, error) {
	p := &routingProblem{input: input, locations: map[string]int{}, orderIndex: map[string]int{}, workerIndex: map[string]int{}, alreadyUsed: map[string]bool{}}
	p.orders = append([]contracts.Order(nil), input.Orders...)
	sort.Slice(p.orders, func(i, j int) bool {
		a, b := p.orders[i], p.orders[j]
		if a.SourceOrder != b.SourceOrder {
			return a.SourceOrder < b.SourceOrder
		}
		return a.ID < b.ID
	})
	for i, o := range p.orders {
		p.orderIndex[o.ID] = i
	}
	for i, id := range input.TravelMatrix.LocationIDs {
		p.locations[id] = i
	}
	for _, id := range input.AlreadyUsedEngineerIDs {
		p.alreadyUsed[id] = true
	}
	states := map[string]contracts.EngineerState{}
	for _, s := range input.EngineerStates {
		states[s.EngineerID] = s
	}
	engineers := append([]contracts.Engineer(nil), input.Engineers...)
	sort.Slice(engineers, func(i, j int) bool { return engineers[i].ID < engineers[j].ID })
	p.origin = math.MaxInt64
	maxEnd := int64(math.MinInt64)
	for _, e := range engineers {
		s := states[e.ID]
		start := shared.Later(e.Shift.Start, s.AvailableFrom).UTC()
		if start.After(e.Shift.End) {
			continue
		}
		skills := map[string]bool{}
		for _, skill := range e.Skills {
			skills[skill] = true
		}
		equipment := map[contracts.Equipment]int64{}
		for k, v := range s.EquipmentAvailable {
			equipment[k] = v
		}
		w := shared.Worker{Engineer: e, Skills: skills, Remaining: equipment, Location: p.locations[s.StartLocationID], InitialLocation: p.locations[s.StartLocationID], Time: start, Route: contracts.Route{EngineerID: e.ID, StartLocationID: s.StartLocationID, StartAt: start, Visits: []contracts.Visit{}, Legs: []contracts.Leg{}}}
		p.workerIndex[e.ID] = len(p.workers)
		p.workers = append(p.workers, w)
		p.origin = min(p.origin, start.Unix())
		maxEnd = max(maxEnd, e.Shift.End.Unix())
	}
	if len(p.workers) == 0 {
		p.origin = 0
		return p, nil
	}
	// Include releases/windows in the epoch to keep every Element value >= 0.
	for _, o := range p.orders {
		p.origin = min(p.origin, o.ReceivedAt.Unix(), o.Window.Start.Unix())
	}
	p.horizon = maxEnd - p.origin
	limit := int64(math.MaxInt64 / 8)
	// Each objective has its own solve stage. Bounds guard native arithmetic,
	// rather than encoding lexicographic priorities in enormous weights.
	if len(p.orders) > 0 && p.horizon > limit/int64(len(p.orders)) {
		return nil, computationError("Сумма временных диапазонов превышает безопасный диапазон int64 OR-Tools")
	}
	var maxDistance int64
	for _, profile := range input.TravelMatrix.Profiles {
		for _, row := range profile {
			for _, cell := range row {
				if cell.Reachable {
					maxDistance = max(maxDistance, *cell.DistanceM)
				}
			}
		}
	}
	if len(p.orders) > 0 && maxDistance > limit/int64(len(p.orders)) {
		return nil, computationError("Верхняя граница суммарного пробега превышает безопасный диапазон int64 OR-Tools")
	}
	for _, kind := range []contracts.Equipment{contracts.EquipmentRouter, contracts.EquipmentTVBox} {
		var total int64
		for _, o := range p.orders {
			n := o.EquipmentRequired[kind]
			if n > limit-total {
				return nil, computationError("Суммарное оборудование превышает безопасный диапазон int64 OR-Tools")
			}
			total += n
		}
	}
	return p, nil
}

type planScore struct {
	counts      [3]int64
	delay       int64
	used        int64
	distance    int64
	assignments []planAssignment
}
type planAssignment struct {
	order    int
	engineer string
	position int
	start    int64
}

func workClass(o contracts.Order) int {
	switch o.WorkType {
	case contracts.WorkTypeEmergency:
		return 0
	case contracts.WorkTypeConnection:
		return 1
	default:
		return 2
	}
}

func (p *routingProblem) score(result contracts.SolveResult) planScore {
	score := planScore{}
	used := map[string]bool{}
	for id := range p.alreadyUsed {
		used[id] = true
	}
	for _, r := range result.Routes {
		if len(r.Visits) > 0 {
			used[r.EngineerID] = true
		}
		for pos, v := range r.Visits {
			idx := p.orderIndex[v.OrderID]
			o := p.orders[idx]
			score.counts[workClass(o)]++
			if o.WorkType == contracts.WorkTypeEmergency {
				score.delay += v.StartAt.Unix() - shared.Later(o.Window.Start, o.ReceivedAt).Unix()
			}
			score.assignments = append(score.assignments, planAssignment{idx, r.EngineerID, pos, v.StartAt.Unix()})
		}
		for _, l := range r.Legs {
			score.distance += l.DistanceM
		}
	}
	score.used = int64(len(used))
	sort.Slice(score.assignments, func(i, j int) bool { return score.assignments[i].order < score.assignments[j].order })
	return score
}

func betterPlan(a, b planScore, emergencyFirst bool) bool {
	if a.counts[0] != b.counts[0] {
		return a.counts[0] > b.counts[0]
	}
	if emergencyFirst && a.delay != b.delay {
		return a.delay < b.delay
	}
	for i := 1; i < len(a.counts); i++ {
		if a.counts[i] != b.counts[i] {
			return a.counts[i] > b.counts[i]
		}
	}
	if !emergencyFirst && a.delay != b.delay {
		return a.delay < b.delay
	}
	if a.used != b.used {
		return a.used < b.used
	}
	if a.distance != b.distance {
		return a.distance < b.distance
	}
	for i, x := range a.assignments {
		y := b.assignments[i]
		if x.order != y.order {
			return x.order < y.order
		}
		if x.engineer != y.engineer {
			return x.engineer < y.engineer
		}
		if x.position != y.position {
			return x.position < y.position
		}
		if x.start != y.start {
			return x.start < y.start
		}
	}
	return false
}

func (p *routingProblem) emptyResult() contracts.SolveResult {
	r := contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}
	for _, o := range p.orders {
		r.Unassigned = append(r.Unassigned, shared.Unassigned(o, contracts.ReasonNotAssignedBySolver, "Поиск не назначил заявку."))
	}
	return r
}

// materialize independently validates a sequence, and creates its earliest
// feasible schedule. In this static, open-route model earlier service never
// worsens feasibility or the emergency-delay objective.
func (p *routingProblem) materialize(routes [][]int) (contracts.SolveResult, error) {
	result := contracts.SolveResult{Routes: []contracts.Route{}, Unassigned: []contracts.UnassignedOrder{}, Termination: contracts.TerminationCompleted}
	seen := make([]bool, len(p.orders))
	legNumber := 0
	if len(routes) != len(p.workers) {
		return result, computationError("OR-Tools вернул неверное количество маршрутов")
	}
	for vehicle, sequence := range routes {
		w := p.workers[vehicle]
		w.Remaining = map[contracts.Equipment]int64{}
		for k, v := range p.workers[vehicle].Remaining {
			w.Remaining[k] = v
		}
		w.Route.Visits = []contracts.Visit{}
		w.Route.Legs = []contracts.Leg{}
		for _, idx := range sequence {
			if idx < 0 || idx >= len(p.orders) || seen[idx] {
				return result, computationError("OR-Tools вернул неизвестную или повторную заявку")
			}
			seen[idx] = true
			o := p.orders[idx]
			if !w.MatchesSkills(o) || !w.MatchesTransport(o) || !w.HasEquipment(o) {
				return result, computationError("OR-Tools нарушил навыки, транспорт или оборудование")
			}
			target := p.locations[o.LocationID]
			cell := p.input.TravelMatrix.Profiles[w.Engineer.Transport][w.Location][target]
			visit, leg, ok := w.AppendCandidate(o, cell)
			if !ok {
				return result, computationError("OR-Tools вернул недопустимое время или недостижимое плечо")
			}
			legNumber++
			leg.ID = fmt.Sprintf("leg-%d", legNumber)
			leg.FromLocationID = p.input.TravelMatrix.LocationIDs[w.Location]
			leg.GeoContextID = p.input.TravelMatrix.GeoContextID
			w.Route.Visits = append(w.Route.Visits, visit)
			w.Route.Legs = append(w.Route.Legs, leg)
			w.Location = target
			w.Time = visit.EndAt
			for k, n := range o.EquipmentRequired {
				w.Remaining[k] -= n
			}
		}
		if len(sequence) > 0 {
			result.Routes = append(result.Routes, w.Route)
		}
	}
	for i, o := range p.orders {
		if !seen[i] {
			result.Unassigned = append(result.Unassigned, shared.Unassigned(o, contracts.ReasonNotAssignedBySolver, "Поиск не назначил заявку."))
		}
	}
	return result, nil
}

func (p *routingProblem) sequences(result contracts.SolveResult) [][]int {
	seq := make([][]int, len(p.workers))
	for _, r := range result.Routes {
		v, ok := p.workerIndex[r.EngineerID]
		if !ok {
			continue
		}
		for _, visit := range r.Visits {
			seq[v] = append(seq[v], p.orderIndex[visit.OrderID])
		}
	}
	return seq
}

func (p *routingProblem) explain(ctx context.Context, result *contracts.SolveResult, deadline time.Time) error {
	stop := func() (bool, error) {
		if err := ctx.Err(); err != nil {
			return true, err
		}
		return !time.Now().Before(deadline), nil
	}
	// Use initial stocks, not the incumbent's remaining equipment. Rearranging
	// other orders may free stock, so a greedy failure is not a proof.
	for i, u := range result.Unassigned {
		reason, done, err := shared.ExplainUnassigned(p.orders[p.orderIndex[u.OrderID]], p.workers, p.locations[p.orders[p.orderIndex[u.OrderID]].LocationID], p.input.TravelMatrix, stop)
		if err != nil {
			return err
		}
		if done {
			for j := i; j < len(result.Unassigned); j++ {
				result.Unassigned[j].Message = "Поиск завершён по лимиту времени. Точная причина неназначения не установлена; невозможность назначения не доказана."
			}
			return nil
		}
		if reason.ReasonCode != contracts.ReasonNotAssignedBySolver {
			result.Unassigned[i] = reason
		} else {
			result.Unassigned[i].Message = p.explainCurrentSchedule(p.orders[p.orderIndex[u.OrderID]], *result, stop)
		}
	}
	return nil
}

// Finish with improving insertions so the bounded global search does not leave
// a usable gap unnoticed. Every candidate is fully validated and must improve
// the same lexicographic objective, including emergency delay priority.
func (p *routingProblem) fillAvailableSlots(ctx context.Context, result contracts.SolveResult, deadline time.Time) (contracts.SolveResult, error) {
	orders := append([]contracts.Order(nil), p.orders...)
	sort.SliceStable(orders, func(i, j int) bool { return workClass(orders[i]) < workClass(orders[j]) })
	for _, order := range orders {
		assigned := true
		for _, item := range result.Unassigned {
			if item.OrderID == order.ID {
				assigned = false
				break
			}
		}
		if assigned {
			continue
		}
		sequences := p.sequences(result)
		best, score := result, p.score(result)
		for vehicle, worker := range p.workers {
			if !worker.MatchesSkills(order) || !worker.MatchesTransport(order) {
				continue
			}
			for pos := 0; pos <= len(sequences[vehicle]); pos++ {
				if err := ctx.Err(); err != nil {
					return result, err
				}
				if !time.Now().Before(deadline) {
					return best, nil
				}
				candidate := append([][]int(nil), sequences...)
				seq := append([]int(nil), sequences[vehicle][:pos]...)
				seq = append(seq, p.orderIndex[order.ID])
				candidate[vehicle] = append(seq, sequences[vehicle][pos:]...)
				value, err := p.materialize(candidate)
				if err != nil {
					continue
				}
				candidateScore := p.score(value)
				if betterPlan(candidateScore, score, p.input.EmergencyFirst) {
					best, score = value, candidateScore
				}
			}
		}
		result = best
	}
	return result, nil
}

// These are constraints of the selected schedule, not a claim that no global
// rearrangement could serve the order. Keep that distinction in the message.
func (p *routingProblem) explainCurrentSchedule(order contracts.Order, result contracts.SolveResult, stop func() (bool, error)) string {
	sequences := p.sequences(result)
	skill, transport, stock, timeBlocked := 0, 0, 0, 0
	for vehicle, worker := range p.workers {
		if !worker.MatchesSkills(order) {
			skill++
			continue
		}
		if !worker.MatchesTransport(order) {
			transport++
			continue
		}
		remaining := map[contracts.Equipment]int64{}
		for k, v := range worker.Remaining {
			remaining[k] = v
		}
		for _, idx := range sequences[vehicle] {
			for k, v := range p.orders[idx].EquipmentRequired {
				remaining[k] -= v
			}
		}
		worker.Remaining = remaining
		if !worker.HasEquipment(order) {
			stock++
			continue
		}
		for pos := 0; pos <= len(sequences[vehicle]); pos++ {
			if done, _ := stop(); done {
				return "Поиск завершён по лимиту времени. Точная причина неназначения не установлена; невозможность назначения не доказана."
			}
			candidate := append([][]int(nil), sequences...)
			seq := append([]int(nil), sequences[vehicle][:pos]...)
			seq = append(seq, p.orderIndex[order.ID])
			candidate[vehicle] = append(seq, sequences[vehicle][pos:]...)
			if value, err := p.materialize(candidate); err == nil {
				if p.input.EmergencyFirst && p.score(value).delay > p.score(result).delay {
					return "Вставка в выбранный маршрут увеличивает опоздание на аварии. В этом варианте минимальное опоздание на аварии важнее добавления обычных заявок."
				}
				return "В текущем расписании есть допустимое место, но ограниченный по времени поиск его не использовал. Повторный расчёт может улучшить результат."
			}
		}
		timeBlocked++
	}
	parts := []string{}
	for _, item := range []struct {
		label string
		count int
	}{
		{"нет нужных навыков", skill}, {"нет нужного транспорта", transport},
		{"не хватает оборудования после назначений", stock},
		{"нет интервала с учётом дороги, окон и смены", timeBlocked},
	} {
		if item.count > 0 {
			parts = append(parts, fmt.Sprintf("%s — %d", item.label, item.count))
		}
	}
	return "По бригадам в выбранном плане: " + strings.Join(parts, "; ") + ". Чтобы добавить заявку, потребуется изменить другие визиты."
}
