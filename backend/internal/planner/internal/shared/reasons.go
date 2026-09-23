package shared

import (
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// A failed append is not a proof that no ordering can serve the order.
// Only emit specific reasons when the corresponding check proves them.
func ExplainUnassigned(order contracts.Order, workers []Worker, target int, matrix contracts.TravelMatrix, stop func() (bool, error)) (contracts.UnassignedOrder, bool, error) {
	skill, transport, equipment, reachable, possibleTime := false, false, false, false, false
	for i := range workers {
		if done, err := stop(); done {
			return contracts.UnassignedOrder{}, true, err
		}
		worker := &workers[i]
		if !worker.MatchesSkills(order) {
			continue
		}
		skill = true
		if !worker.MatchesTransport(order) {
			continue
		}
		transport = true
		if !worker.HasEquipment(order) {
			continue
		}
		equipment = true
		if worker.reachable == nil {
			seen, done, err := connectedLocations(matrix.Profiles[worker.Engineer.Transport], worker.InitialLocation, stop)
			if done {
				return contracts.UnassignedOrder{}, true, err
			}
			worker.reachable = seen
		}
		if !worker.reachable[target] {
			continue
		}
		reachable = true
		// Ignore all travel and other future jobs. If even this optimistic
		// start cannot fit, rearranging the route cannot create a slot.
		start := Later(Later(worker.Route.StartAt, order.ReceivedAt), order.Window.Start)
		if !start.After(order.Window.End) && !start.After(worker.Engineer.Shift.End) &&
			time.Duration(order.ServiceSec)*time.Second <= worker.Engineer.Shift.End.Sub(start) {
			possibleTime = true
		}
	}
	code, message := contracts.ReasonNotAssignedBySolver, "Baseline не нашёл назначения без изменения ранее построенных маршрутов."
	switch {
	case len(workers) == 0:
		code, message = contracts.ReasonNoAvailableEngineer, "Нет доступных инженеров."
	case !skill:
		code, message = contracts.ReasonNoMatchingSkill, "Нет инженера со всеми необходимыми навыками."
	case !transport:
		code, message = contracts.ReasonNoMatchingTransport, "У инженеров с нужными навыками нет требуемого транспорта."
	case !equipment:
		code, message = contracts.ReasonNoMatchingEquipment, "Недостаточно оборудования у подходящих инженеров с учётом уже назначенных заявок."
	case !reachable:
		code, message = contracts.ReasonNoReachableRoute, "Точка заявки недостижима из стартовых точек подходящих инженеров."
	case !possibleTime:
		code, message = contracts.ReasonNoFeasibleSlot, "Работа не укладывается в окно начала и доступную смену даже без времени дороги."
	}
	return Unassigned(order, code, message), false, nil
}

// Connectivity is deliberately optimistic: intermediate matrix locations need
// not be serviceable jobs. Lack of a path still proves impossibility, whereas a
// missing direct edge alone does not (the matrix need not satisfy metric laws).
func connectedLocations(matrix [][]contracts.TravelCell, start int, stop func() (bool, error)) ([]bool, bool, error) {
	seen := make([]bool, len(matrix))
	seen[start] = true
	queue := []int{start}
	for head := 0; head < len(queue); head++ {
		if done, err := stop(); done {
			return nil, true, err
		}
		for to, cell := range matrix[queue[head]] {
			if cell.Reachable && !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen, false, nil
}
