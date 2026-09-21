package planner

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

// maxDurationSec keeps seconds-to-time.Duration conversions from overflowing.
const maxDurationSec = int64(math.MaxInt64 / int64(time.Second))

func invalidInput(field, message string) error {
	return &contracts.ContractError{
		Code:    "INVALID_INPUT",
		Message: message,
		Details: map[string]any{"field": field},
	}
}

func validateInput(ctx context.Context, in contracts.SolveRequest) error {
	if in.Mode != contracts.SolveModeBaseline {
		return invalidInput("mode", "поддерживается только baseline")
	}
	if in.TimeLimitMS <= 0 || in.TimeLimitMS > math.MaxInt64/int64(time.Millisecond) {
		return invalidInput("time_limit_ms", "лимит должен быть положительным и допустимым для time.Duration")
	}

	matrix := in.TravelMatrix
	if matrix.ID == "" {
		return invalidInput("travel_matrix.id", "id не должен быть пустым")
	}
	if matrix.GeoContextID == "" {
		return invalidInput("travel_matrix.geo_context_id", "geo_context_id не должен быть пустым")
	}

	locationIndexes := make(map[string]int, len(matrix.LocationIDs))
	for i, id := range matrix.LocationIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		field := fmt.Sprintf("travel_matrix.location_ids[%d]", i)
		if id == "" {
			return invalidInput(field, "id не должен быть пустым")
		}
		if _, exists := locationIndexes[id]; exists {
			return invalidInput(field, "id должен быть уникальным")
		}
		locationIndexes[id] = i
	}

	for transport := range matrix.Profiles {
		if !validTransport(transport) {
			return invalidInput("travel_matrix.profiles."+string(transport), "неизвестный транспорт")
		}
	}
	for _, transport := range []contracts.Transport{contracts.TransportCar, contracts.TransportWalk} {
		if err := ctx.Err(); err != nil {
			return err
		}
		profile, ok := matrix.Profiles[transport]
		if !ok {
			continue
		}
		n := len(matrix.LocationIDs)
		if len(profile) != n {
			return invalidInput("travel_matrix.profiles."+string(transport), "матрица должна быть квадратной N×N")
		}
		for i, row := range profile {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(row) != n {
				return invalidInput(fmt.Sprintf("travel_matrix.profiles.%s[%d]", transport, i), "строка должна иметь размер N")
			}
			for j, cell := range row {
				if err := ctx.Err(); err != nil {
					return err
				}
				field := fmt.Sprintf("travel_matrix.profiles.%s[%d][%d]", transport, i, j)
				if cell.Reachable {
					if cell.DurationSec == nil || cell.DistanceM == nil {
						return invalidInput(field, "достижимая ячейка требует duration_sec и distance_m")
					}
					if *cell.DurationSec < 0 || *cell.DurationSec > maxDurationSec {
						return invalidInput(field+".duration_sec", "значение вне допустимого диапазона")
					}
					if *cell.DistanceM < 0 {
						return invalidInput(field+".distance_m", "расстояние не может быть отрицательным")
					}
				} else if cell.DurationSec != nil || cell.DistanceM != nil {
					return invalidInput(field, "недостижимая ячейка должна иметь null duration_sec и distance_m")
				}
				if i == j && (!cell.Reachable || *cell.DurationSec != 0 || *cell.DistanceM != 0) {
					return invalidInput(field, "диагональ должна быть достижимой с нулевыми значениями")
				}
			}
		}
	}

	usedIDs := make(map[string]struct{}, len(in.AlreadyUsedEngineerIDs))
	for i, id := range in.AlreadyUsedEngineerIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		field := fmt.Sprintf("already_used_engineer_ids[%d]", i)
		if id == "" {
			return invalidInput(field, "id не должен быть пустым")
		}
		if _, exists := usedIDs[id]; exists {
			return invalidInput(field, "id должен быть уникальным")
		}
		usedIDs[id] = struct{}{}
	}

	engineerIDs := make(map[string]contracts.Engineer, len(in.Engineers))
	for i, engineer := range in.Engineers {
		if err := ctx.Err(); err != nil {
			return err
		}
		field := fmt.Sprintf("engineers[%d]", i)
		if engineer.ID == "" {
			return invalidInput(field+".id", "id не должен быть пустым")
		}
		if _, exists := engineerIDs[engineer.ID]; exists {
			return invalidInput(field+".id", "id должен быть уникальным")
		}
		engineerIDs[engineer.ID] = engineer
		if !engineer.Available {
			return invalidInput(field+".available", "переданные инженеры должны быть доступны")
		}
		if !validTransport(engineer.Transport) {
			return invalidInput(field+".transport", "неизвестный транспорт")
		}
		if err := validateWindow(field+".shift", engineer.Shift); err != nil {
			return err
		}
		for j, skill := range engineer.Skills {
			if err := ctx.Err(); err != nil {
				return err
			}
			if skill == "" {
				return invalidInput(fmt.Sprintf("%s.skills[%d]", field, j), "skill не должен быть пустым")
			}
		}
		if err := validateEquipmentMap(field+".equipment_stock", engineer.EquipmentStock); err != nil {
			return err
		}
	}

	states := make(map[string]contracts.EngineerState, len(in.EngineerStates))
	for i, state := range in.EngineerStates {
		if err := ctx.Err(); err != nil {
			return err
		}
		field := fmt.Sprintf("engineer_states[%d]", i)
		if state.EngineerID == "" {
			return invalidInput(field+".engineer_id", "engineer_id не должен быть пустым")
		}
		if _, exists := engineerIDs[state.EngineerID]; !exists {
			return invalidInput(field+".engineer_id", "state ссылается на неизвестного инженера")
		}
		if _, exists := states[state.EngineerID]; exists {
			return invalidInput(field+".engineer_id", "для инженера допускается ровно один state")
		}
		states[state.EngineerID] = state
		if state.StartLocationID == "" {
			return invalidInput(field+".start_location_id", "location id не должен быть пустым")
		}
		if _, exists := locationIndexes[state.StartLocationID]; !exists {
			return invalidInput(field+".start_location_id", "стартовая точка отсутствует в travel_matrix")
		}
		if err := validateTime(field+".available_from", state.AvailableFrom); err != nil {
			return err
		}
		if err := validateEquipmentMap(field+".equipment_available", state.EquipmentAvailable); err != nil {
			return err
		}
		for equipment, available := range state.EquipmentAvailable {
			if available > engineerIDs[state.EngineerID].EquipmentStock[equipment] {
				return invalidInput(field+".equipment_available."+string(equipment), "остаток превышает equipment_stock")
			}
		}
	}
	for i, engineer := range in.Engineers {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, exists := states[engineer.ID]; !exists {
			return invalidInput(fmt.Sprintf("engineer_states[%d]", i), "для инженера требуется ровно один state")
		}
		if _, exists := matrix.Profiles[engineer.Transport]; !exists {
			return invalidInput("travel_matrix.profiles."+string(engineer.Transport), "профиль инженера отсутствует")
		}
	}

	orderIDs := make(map[string]struct{}, len(in.Orders))
	for i, order := range in.Orders {
		if err := ctx.Err(); err != nil {
			return err
		}
		field := fmt.Sprintf("orders[%d]", i)
		if order.ID == "" {
			return invalidInput(field+".id", "id не должен быть пустым")
		}
		if _, exists := orderIDs[order.ID]; exists {
			return invalidInput(field+".id", "id должен быть уникальным")
		}
		orderIDs[order.ID] = struct{}{}
		if order.LocationID == "" {
			return invalidInput(field+".location_id", "location id не должен быть пустым")
		}
		if _, exists := locationIndexes[order.LocationID]; !exists {
			return invalidInput(field+".location_id", "точка заявки отсутствует в travel_matrix")
		}
		if err := validateTime(field+".received_at", order.ReceivedAt); err != nil {
			return err
		}
		if err := validateWindow(field+".window", order.Window); err != nil {
			return err
		}
		if order.ServiceSec <= 0 || order.ServiceSec > maxDurationSec {
			return invalidInput(field+".service_sec", "длительность должна быть положительной и допустимой для time.Duration")
		}
		if err := validateOrderType(field, order); err != nil {
			return err
		}
		if order.RequiredTransport != nil && !validTransport(*order.RequiredTransport) {
			return invalidInput(field+".required_transport", "неизвестный транспорт")
		}
		for j, skill := range order.RequiredSkills {
			if err := ctx.Err(); err != nil {
				return err
			}
			if skill == "" {
				return invalidInput(fmt.Sprintf("%s.required_skills[%d]", field, j), "skill не должен быть пустым")
			}
		}
		if err := validateEquipmentMap(field+".equipment_required", order.EquipmentRequired); err != nil {
			return err
		}
		switch order.Status {
		case contracts.OrderStatusActive:
			if order.Execution != nil {
				return invalidInput(field+".execution", "у active execution должен быть null")
			}
		case contracts.OrderStatusSent, contracts.OrderStatusEnRoute:
			execution := order.Execution
			if execution == nil || execution.EngineerID == "" {
				return invalidInput(field+".execution.engineer_id", "sent и en_route требуют исполнителя")
			}
			if execution.StartedAt != nil || execution.FinishedAt != nil || execution.ExpectedEndAt != nil {
				return invalidInput(field+".execution", "будущая заявка не должна содержать started_at, finished_at или expected_end_at")
			}
			if order.Status == contracts.OrderStatusSent {
				if execution.DepartedAt != nil {
					return invalidInput(field+".execution.departed_at", "у sent должен быть null")
				}
			} else {
				if execution.DepartedAt == nil {
					return invalidInput(field+".execution.departed_at", "у en_route требуется время выезда")
				}
				if err := validateTime(field+".execution.departed_at", *execution.DepartedAt); err != nil {
					return err
				}
				if execution.DepartedAt.Before(order.ReceivedAt) {
					return invalidInput(field+".execution.departed_at", "выезд не может быть раньше received_at")
				}
			}
		default:
			return invalidInput(field+".status", "допустимы только active, sent или en_route")
		}
	}

	return nil
}

func validateOrderType(field string, order contracts.Order) error {
	switch order.WorkType {
	case contracts.WorkTypeEmergency:
		if order.Priority != contracts.PriorityUrgent {
			return invalidInput(field+".priority", "emergency требует priority=urgent")
		}
		if order.ServiceSec != 4800 {
			return invalidInput(field+".service_sec", "emergency требует service_sec=4800")
		}
	case contracts.WorkTypeConnection, contracts.WorkTypeRepair, contracts.WorkTypeAdditional:
		if order.Priority != contracts.PriorityNormal {
			return invalidInput(field+".priority", "обычная работа требует priority=normal")
		}
	default:
		return invalidInput(field+".work_type", "неизвестный тип работы")
	}
	return nil
}

func validateEquipmentMap(field string, equipment map[contracts.Equipment]int64) error {
	for kind, count := range equipment {
		if kind != contracts.EquipmentRouter && kind != contracts.EquipmentTVBox {
			return invalidInput(field+"."+string(kind), "неизвестный тип оборудования")
		}
		if count < 0 {
			return invalidInput(field+"."+string(kind), "количество не может быть отрицательным")
		}
	}
	return nil
}

func validateWindow(field string, window contracts.Window) error {
	if err := validateTime(field+".start", window.Start); err != nil {
		return err
	}
	if err := validateTime(field+".end", window.End); err != nil {
		return err
	}
	if window.Start.After(window.End) {
		return invalidInput(field, "start не может быть позже end")
	}
	return nil
}

func validateTime(field string, value time.Time) error {
	if value.IsZero() || value.Year() < 1 || value.Year() > 9999 || value.UTC().Year() < 1 || value.UTC().Year() > 9999 {
		return invalidInput(field, "время должно быть задано и представимо в RFC3339")
	}
	if value.Nanosecond() != 0 {
		return invalidInput(field, "точность времени должна быть до секунды")
	}
	_, offset := value.Zone()
	if offset%60 != 0 || offset < -(23*60+59)*60 || offset > (23*60+59)*60 {
		return invalidInput(field, "смещение часового пояса не представимо в RFC3339")
	}
	return nil
}

func validTransport(transport contracts.Transport) bool {
	return transport == contracts.TransportCar || transport == contracts.TransportWalk
}
