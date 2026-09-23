package plans

import (
	"sort"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

func calculateMetrics(routes []contracts.Route, unassigned []contracts.UnassignedOrder, cancelledIDs, completedIDs []string) contracts.Metrics {
	cancelled := make(map[string]struct{}, len(cancelledIDs))
	for _, orderID := range cancelledIDs {
		cancelled[orderID] = struct{}{}
	}
	assignments := make(map[string]struct{})
	distanceByEngineer := make(map[string]int64)
	used := make(map[string]struct{})
	for _, route := range routes {
		if len(route.Visits) > 0 || len(route.Legs) > 0 {
			used[route.EngineerID] = struct{}{}
		}
		for _, visit := range route.Visits {
			if _, isCancelled := cancelled[visit.OrderID]; isCancelled {
				continue
			}
			assignments[visit.OrderID] = struct{}{}
		}
		for _, leg := range route.Legs {
			distanceByEngineer[route.EngineerID] += leg.DistanceM
		}
	}
	engineerIDs := make([]string, 0, len(distanceByEngineer))
	for engineerID := range distanceByEngineer {
		engineerIDs = append(engineerIDs, engineerID)
	}
	sort.Strings(engineerIDs)
	perEngineer := make([]contracts.EngineerDistance, 0, len(engineerIDs))
	var total int64
	for _, engineerID := range engineerIDs {
		distance := distanceByEngineer[engineerID]
		total += distance
		perEngineer = append(perEngineer, contracts.EngineerDistance{EngineerID: engineerID, DistanceM: distance})
	}
	return contracts.Metrics{
		AssignedCount:     len(assignments),
		CompletedCount:    len(completedIDs),
		UnassignedCount:   len(unassigned),
		UsedEngineerCount: len(used),
		TotalDistanceM:    total,
		PerEngineer:       perEngineer,
	}
}
