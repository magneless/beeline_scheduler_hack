package plans

import "github.com/magneless/beeline_scheduler_hack/backend/contracts"

func equipmentRemaining(s contracts.Snapshot) (map[string]map[contracts.Equipment]int64, error) {
	result := map[string]map[contracts.Equipment]int64{}
	for _, eng := range s.Engineers {
		result[eng.ID] = cloneEquipment(eng.EquipmentStock)
	}
	for _, o := range s.Orders {
		if o.Execution == nil || o.Execution.StartedAt == nil {
			continue
		}
		stock, ok := result[o.Execution.EngineerID]
		if !ok {
			return nil, contracts.InvalidInput("execution references unknown engineer", nil)
		}
		for kind, n := range o.EquipmentRequired {
			stock[kind] -= n
			if stock[kind] < 0 {
				return nil, contracts.InvalidInput("negative equipment balance", nil)
			}
		}
	}
	return result, nil
}
func cloneEquipment(m map[contracts.Equipment]int64) map[contracts.Equipment]int64 {
	out := map[contracts.Equipment]int64{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
