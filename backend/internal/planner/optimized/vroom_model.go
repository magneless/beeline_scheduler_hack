package optimized

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// VROOM 1.15 API: https://github.com/VROOM-Project/vroom/blob/v1.15.0/docs/API.md
// The model deliberately uses custom matrices and open routes: no routing
// server, coordinates, or return-to-depot leg is needed by the subprocess.
type vroomInput struct {
	Jobs     []vroomJob                          `json:"jobs"`
	Vehicles []vroomVehicle                      `json:"vehicles"`
	Matrices map[contracts.Transport]vroomMatrix `json:"matrices"`
}

type vroomJob struct {
	ID            int        `json:"id"`
	LocationIndex int        `json:"location_index"`
	Service       int64      `json:"service"`
	Delivery      [2]int64   `json:"delivery"`
	Skills        []int      `json:"skills"`
	Priority      int        `json:"priority"`
	TimeWindows   [][2]int64 `json:"time_windows"`
}

type vroomVehicle struct {
	ID         int                 `json:"id"`
	Profile    contracts.Transport `json:"profile"`
	StartIndex int                 `json:"start_index"`
	Capacity   [2]int64            `json:"capacity"`
	Skills     []int               `json:"skills"`
	TimeWindow [2]int64            `json:"time_window"`
	Costs      vroomCosts          `json:"costs"`
}

type vroomCosts struct {
	Fixed int64 `json:"fixed"`
}

type vroomMatrix struct {
	Durations [][]int64 `json:"durations"`
	Distances [][]int64 `json:"distances"`
	Costs     [][]int64 `json:"costs"`
}

func (p *routingProblem) vroomRequest(ctx context.Context) (vroomInput, error) {
	input := vroomInput{Jobs: []vroomJob{}, Vehicles: []vroomVehicle{}, Matrices: map[contracts.Transport]vroomMatrix{}}
	// VROOM uses uint16 ranks and uint32 user times/distances/costs. Do not
	// silently round, wrap, or weaken constraints for out-of-range requests.
	if len(p.orders) > math.MaxUint16 || len(p.workers) > math.MaxUint16 || len(p.locations) > math.MaxUint16 || p.horizon >= math.MaxUint32 {
		return input, computationError("Размер задачи или временной горизонт превышает диапазон VROOM")
	}
	skillSet := map[string]bool{}
	for _, worker := range p.workers {
		for skill := range worker.Skills {
			skillSet[skill] = true
		}
	}
	for _, order := range p.orders {
		for _, skill := range order.RequiredSkills {
			skillSet[skill] = true
		}
	}
	names := make([]string, 0, len(skillSet))
	for name := range skillSet {
		names = append(names, name)
	}
	sort.Strings(names)
	skills := map[string]int{}
	for i, name := range names {
		skills[name] = i + 1
	}
	// Transport capabilities live in a separate namespace from user skills.
	transportSkills := map[contracts.Transport]int{contracts.TransportCar: len(skills) + 1, contracts.TransportWalk: len(skills) + 2}
	for i, order := range p.orders {
		if err := ctx.Err(); err != nil {
			return input, err
		}
		lower := max(order.Window.Start.Unix(), order.ReceivedAt.Unix()) - p.origin
		upper := min(order.Window.End.Unix()-p.origin, p.horizon)
		if lower > upper || order.ServiceSec > p.horizon {
			continue // Globally outside the shifts; remains unassigned in Go.
		}
		job := vroomJob{ID: i + 1, LocationIndex: p.locations[order.LocationID], Service: order.ServiceSec,
			Delivery: equipmentAmounts(order.EquipmentRequired), Skills: []int{}, Priority: 1, TimeWindows: [][2]int64{{lower, upper}}}
		for _, skill := range order.RequiredSkills {
			job.Skills = append(job.Skills, skills[skill])
		}
		if order.RequiredTransport != nil {
			job.Skills = append(job.Skills, transportSkills[*order.RequiredTransport])
		}
		input.Jobs = append(input.Jobs, job)
	}
	var maxDistance int64
	for _, worker := range p.workers {
		profile := worker.Engineer.Transport
		if _, exists := input.Matrices[profile]; exists {
			continue
		}
		cells := p.input.TravelMatrix.Profiles[profile]
		matrix := vroomMatrix{Durations: make([][]int64, len(cells)), Distances: make([][]int64, len(cells))}
		for from, row := range cells {
			if err := ctx.Err(); err != nil {
				return input, err
			}
			matrix.Durations[from], matrix.Distances[from] = make([]int64, len(row)), make([]int64, len(row))
			for to, cell := range row {
				if !cell.Reachable {
					matrix.Durations[from][to] = p.horizon + 1
					continue
				}
				if *cell.DistanceM > math.MaxUint32 {
					return input, computationError("Расстояние превышает диапазон VROOM")
				}
				// A trip longer than the entire horizon is infeasible for every
				// vehicle; its precise (possibly >uint32) duration is irrelevant.
				matrix.Durations[from][to] = min(*cell.DurationSec, p.horizon+1)
				matrix.Distances[from][to] = *cell.DistanceM
				maxDistance = max(maxDistance, *cell.DistanceM)
			}
		}
		matrix.Costs = matrix.Distances
		input.Matrices[profile] = matrix
	}
	// This weight encodes only crew count before distance. All other criteria
	// (including emergency delay before crew count) are compared in Go.
	fixed := int64(len(p.orders))*maxDistance + 1
	if fixed > math.MaxUint32/int64(len(p.workers)+1) {
		return input, computationError("Суммарная стоимость маршрутов превышает диапазон VROOM")
	}
	for i, worker := range p.workers {
		vehicle := vroomVehicle{ID: i + 1, Profile: worker.Engineer.Transport, StartIndex: worker.InitialLocation,
			Capacity: equipmentAmounts(worker.Remaining), Skills: []int{transportSkills[worker.Engineer.Transport]},
			TimeWindow: [2]int64{worker.Time.Unix() - p.origin, worker.Engineer.Shift.End.Unix() - p.origin}}
		for _, skill := range worker.Engineer.Skills {
			vehicle.Skills = append(vehicle.Skills, skills[skill])
		}
		if !p.alreadyUsed[worker.Engineer.ID] {
			vehicle.Costs.Fixed = fixed
		}
		input.Vehicles = append(input.Vehicles, vehicle)
	}
	return input, nil
}

func equipmentAmounts(stock map[contracts.Equipment]int64) [2]int64 {
	return [2]int64{stock[contracts.EquipmentRouter], stock[contracts.EquipmentTVBox]}
}

type vroomOutput struct {
	Code       *int         `json:"code"`
	Error      string       `json:"error"`
	Routes     []vroomRoute `json:"routes"`
	Unassigned []vroomStep  `json:"unassigned"`
}

type vroomRoute struct {
	Vehicle    int               `json:"vehicle"`
	Steps      []vroomStep       `json:"steps"`
	Violations []json.RawMessage `json:"violations"`
}

type vroomStep struct {
	Type       string            `json:"type"`
	ID         int               `json:"id"`
	Violations []json.RawMessage `json:"violations"`
}

// Decode identities and complete task accounting before using any candidate.
// Native timestamps are deliberately ignored: materialize applies received_at
// to departure, which is a stricter rule than VROOM's service time windows.
func (p *routingProblem) vroomSequences(input vroomInput, data []byte) ([][]int, error) {
	var output vroomOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, computationError(fmt.Sprintf("Некорректный JSON VROOM: %v", err))
	}
	if output.Code == nil {
		return nil, computationError("В ответе VROOM отсутствует code")
	}
	if *output.Code != 0 || output.Error != "" {
		return nil, computationError(fmt.Sprintf("VROOM code=%d: %s", *output.Code, output.Error))
	}
	jobs := make(map[int]bool, len(input.Jobs))
	for _, job := range input.Jobs {
		jobs[job.ID] = false
	}
	mark := func(step vroomStep) error {
		seen, known := jobs[step.ID]
		if step.Type != "job" || !known || seen || len(step.Violations) != 0 {
			return computationError("VROOM вернул неизвестную, повторную заявку или нарушение ограничений")
		}
		jobs[step.ID] = true
		return nil
	}
	sequences := make([][]int, len(p.workers))
	vehicles := map[int]bool{}
	for _, route := range output.Routes {
		if route.Vehicle < 1 || route.Vehicle > len(p.workers) || vehicles[route.Vehicle] || len(route.Violations) != 0 {
			return nil, computationError("VROOM вернул неизвестного, повторного инженера или нарушение ограничений")
		}
		vehicles[route.Vehicle] = true
		for _, step := range route.Steps {
			if len(step.Violations) != 0 {
				return nil, computationError("VROOM вернул нарушение ограничений на участке маршрута")
			}
			if step.Type == "start" || step.Type == "end" {
				continue
			}
			if err := mark(step); err != nil {
				return nil, err
			}
			sequences[route.Vehicle-1] = append(sequences[route.Vehicle-1], step.ID-1)
		}
	}
	for _, step := range output.Unassigned {
		if err := mark(step); err != nil {
			return nil, err
		}
	}
	for _, seen := range jobs {
		if !seen {
			return nil, computationError("В ответе VROOM потеряна заявка")
		}
	}
	return sequences, nil
}
