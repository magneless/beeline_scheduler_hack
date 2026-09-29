package plans

import (
	"context"
	"reflect"
	"sort"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

type dispatchPolicy struct {
	key          string
	lateIDs      map[string]bool
	reserve      bool
	wasAvailable map[string]bool
	recruited    []string
}

func lateEligible(strict contracts.PlanResult, snapshot *contracts.Snapshot) map[string]bool {
	result := map[string]bool{}
	orders := orderMap(strict.TargetSnapshot.Orders)
	if snapshot != nil {
		orders = orderMap(snapshot.Orders)
	}
	for _, item := range strict.Draft.Unassigned {
		order := orders[item.OrderID]
		if order.WorkType == contracts.WorkTypeEmergency && order.Priority == contracts.PriorityUrgent {
			result[item.OrderID] = true
		}
	}
	return result
}

func widenEmergencyWindows(orders []contracts.Order, eligible map[string]bool, engineers []contracts.Engineer) []contracts.Order {
	out := cloneOrders(orders)
	for i := range out {
		order := &out[i]
		if !eligible[order.ID] || order.WorkType != contracts.WorkTypeEmergency || order.Priority != contracts.PriorityUrgent {
			continue
		}
		for _, engineer := range engineers {
			if !engineer.Available || !hasAllSkills(engineer.Skills, order.RequiredSkills) || (order.RequiredTransport != nil && *order.RequiredTransport != engineer.Transport) {
				continue
			}
			if engineer.Shift.End.After(order.Window.End) {
				order.Window.End = engineer.Shift.End
			}
		}
	}
	return out
}

func measureLateness(snapshot contracts.Snapshot, routes []contracts.Route) []contracts.OrderLateness {
	orders := orderMap(snapshot.Orders)
	var out []contracts.OrderLateness
	for _, route := range routes {
		for _, visit := range route.Visits {
			order := orders[visit.OrderID]
			if order.WorkType != contracts.WorkTypeEmergency || order.Priority != contracts.PriorityUrgent || !visit.StartAt.After(order.Window.End) || order.Execution != nil && order.Execution.StartedAt != nil {
				continue
			}
			out = append(out, contracts.OrderLateness{OrderID: order.ID, Window: order.Window, ArrivalAt: visit.ArrivalAt, StartAt: visit.StartAt, LateSec: int64(visit.StartAt.Sub(order.Window.End) / time.Second)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrderID < out[j].OrderID })
	return out
}

func carryAcceptedLateness(snapshot contracts.Snapshot, routes []contracts.Route, accepted []contracts.OrderLateness) []contracts.OrderLateness {
	out := measureLateness(snapshot, routes)
	orders := orderMap(snapshot.Orders)
	for _, item := range accepted {
		order := orders[item.OrderID]
		if order.Execution == nil || order.Execution.StartedAt == nil {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrderID < out[j].OrderID })
	return out
}

func snapshotEngineerAvailable(snapshot contracts.Snapshot, id string) bool {
	for _, engineer := range snapshot.Engineers {
		if engineer.ID == id {
			return engineer.Available
		}
	}
	return false
}

func plannedEnd(routes []contracts.Route, orderID string) time.Time {
	for _, route := range routes {
		for _, visit := range route.Visits {
			if visit.OrderID == orderID {
				return visit.EndAt
			}
		}
	}
	return time.Time{}
}

func deferredOrderIDs(items []contracts.UnassignedOrder) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.OrderID)
	}
	sort.Strings(ids)
	return ids
}

func option(key, label string, result contracts.PlanResult, earlier []contracts.PlanOption) contracts.PlanOption {
	result.Draft.OptionKey = key
	out := contracts.PlanOption{Key: key, Label: label, Result: result, Lateness: append([]contracts.OrderLateness{}, result.Draft.Lateness...), ReserveEngineerIDs: append([]string{}, result.Draft.ReserveEngineerIDs...)}
	for _, previous := range earlier {
		if reflect.DeepEqual(semanticRoutes(previous.Result.Draft.Routes), semanticRoutes(result.Draft.Routes)) && reflect.DeepEqual(previous.Result.Draft.Unassigned, result.Draft.Unassigned) && reflect.DeepEqual(previous.Result.Draft.Lateness, result.Draft.Lateness) && reflect.DeepEqual(previous.Result.Draft.ReserveEngineerIDs, result.Draft.ReserveEngineerIDs) {
			out.IdenticalTo = &previous.Key
			break
		}
	}
	return out
}

func semanticRoutes(routes []contracts.Route) []contracts.Route {
	copy := clonePlanRoutes(routes)
	for i := range copy {
		for j := range copy[i].Legs {
			copy[i].Legs[j].ID = ""
			copy[i].Legs[j].GeoContextID = ""
		}
	}
	return copy
}

func (service *Service) BuildOptions(ctx context.Context, input contracts.BuildPlanRequest) ([]contracts.PlanOption, error) {
	strictCtx := progress.WithState(ctx, progress.State{Completed: 0, Total: 2, Label: "С соблюдением окон"})
	progress.Report(strictCtx, "variant", "Рассчитываем маршрут с соблюдением окон")
	input.OptionKey = "strict"
	strict, err := service.Build(strictCtx, input)
	if err != nil {
		return nil, err
	}
	options := []contracts.PlanOption{option("strict", "С соблюдением всех окон", strict, nil)}
	progress.Report(progress.WithState(ctx, progress.State{Completed: 1, Total: 2, Label: "С соблюдением окон"}), "variant_complete", "С соблюдением окон готов")
	lateCtx := progress.WithState(ctx, progress.State{Completed: 1, Total: 2, Label: "Маршрут с опозданием на аварии"})
	progress.Report(lateCtx, "variant", "Рассчитываем допуск опоздания на аварии")
	input.OptionKey = "late_emergency"
	late := strict
	if eligible := lateEligible(strict, nil); len(eligible) > 0 {
		late, err = service.buildWithPolicy(lateCtx, input, dispatchPolicy{key: "late_emergency", lateIDs: eligible})
		if err != nil {
			return nil, err
		}
	}
	progress.Report(progress.WithState(ctx, progress.State{Completed: 2, Total: 2, Label: "Маршрут с опозданием на аварии"}), "variant_complete", "Варианты рассчитаны")
	return append(options, option("late_emergency", "С допуском опоздания на аварии", late, options)), nil
}

func (service *Service) ReplanOptions(ctx context.Context, input contracts.ReplanRequest) ([]contracts.PlanOption, error) {
	strictCtx := progress.WithState(ctx, progress.State{Completed: 0, Total: 3, Label: "С соблюдением окон"})
	progress.Report(strictCtx, "variant", "Рассчитываем маршрут с соблюдением окон")
	input.OptionKey = "strict"
	strict, err := service.Replan(strictCtx, input)
	if err != nil {
		return nil, err
	}
	options := []contracts.PlanOption{option("strict", "С работающими инженерами", strict, nil)}
	progress.Report(progress.WithState(ctx, progress.State{Completed: 1, Total: 3, Label: "С соблюдением окон"}), "variant_complete", "С соблюдением окон готов")
	if eventIncludes(input.Event, contracts.EventUrgentOrderAdded) {
		lateCtx := progress.WithState(ctx, progress.State{Completed: 1, Total: 3, Label: "Маршрут с опозданием на аварию"})
		progress.Report(lateCtx, "variant", "Рассчитываем допуск опоздания на аварию")
		input.OptionKey = "late_emergency"
		late := strict
		if eligible := lateEligible(strict, nil); len(eligible) > 0 {
			late, err = service.replanWithPolicy(lateCtx, input, dispatchPolicy{key: "late_emergency", lateIDs: eligible})
			if err != nil {
				return nil, err
			}
		}
		options = append(options, option("late_emergency", "С допуском опоздания на аварию", late, options))
	} else {
		key, label := "original", "Сохранить расписание"
		if eventIncludes(input.Event, contracts.EventEngineerUnavailable) {
			key, label = "remove_unavailable", "Снять оставшиеся заявки недоступных бригад"
		}
		originalCtx := progress.WithState(ctx, progress.State{Completed: 1, Total: 3, Label: label})
		progress.Report(originalCtx, "variant", label)
		input.OptionKey = key
		original, err := service.Replan(originalCtx, input)
		if err != nil {
			return nil, err
		}
		options = append(options, option(key, label, original, options))
	}
	progress.Report(progress.WithState(ctx, progress.State{Completed: 2, Total: 3}), "variant_complete", "Второй вариант готов")
	reserveCtx := progress.WithState(ctx, progress.State{Completed: 2, Total: 3, Label: "Маршрут с резервом"})
	progress.Report(reserveCtx, "variant", "Рассчитываем резервных инженеров")
	input.OptionKey = "reserve"
	reserve := strict
	hasReserve := false
	for _, engineer := range strict.TargetSnapshot.Engineers {
		hasReserve = hasReserve || engineer.Reserve
	}
	if hasReserve {
		reserve, err = service.Replan(reserveCtx, input)
		if err != nil {
			return nil, err
		}
	}
	progress.Report(progress.WithState(ctx, progress.State{Completed: 3, Total: 3, Label: "Маршрут с резервом"}), "variant_complete", "Варианты рассчитаны")
	return append(options, option("reserve", "С привлечением резерва", reserve, options)), nil
}
