package plans

import (
	"sort"

	"github.com/magneless/beeline_scheduler_hack/contracts"
)

func calculateChanges(before, after []contracts.Route, cancelled map[string]struct{}) []contracts.PlanChange {
	beforeAssignments := assignments(before)
	afterAssignments := assignments(after)
	for id := range cancelled {
		delete(afterAssignments, id)
	}
	orderIDs := make(map[string]struct{}, len(beforeAssignments)+len(afterAssignments)+len(cancelled))
	for id := range beforeAssignments {
		orderIDs[id] = struct{}{}
	}
	for id := range afterAssignments {
		orderIDs[id] = struct{}{}
	}
	for id := range cancelled {
		orderIDs[id] = struct{}{}
	}
	ids := make([]string, 0, len(orderIDs))
	for id := range orderIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	changes := make([]contracts.PlanChange, 0)
	for _, id := range ids {
		beforeAssignment, hadBefore := beforeAssignments[id]
		afterAssignment, hasAfter := afterAssignments[id]
		if hadBefore && hasAfter && assignmentEqual(beforeAssignment, afterAssignment) {
			continue
		}
		if !hadBefore && !hasAfter {
			if _, isCancelled := cancelled[id]; isCancelled {
				changes = append(changes, contracts.PlanChange{OrderID: id, Reason: contracts.PlanChangeCancelled})
			}
			continue
		}
		change := contracts.PlanChange{OrderID: id}
		if hadBefore {
			copyAssignment := beforeAssignment
			change.Before = &copyAssignment
		}
		if hasAfter {
			copyAssignment := afterAssignment
			change.After = &copyAssignment
		}
		if _, isCancelled := cancelled[id]; isCancelled {
			change.Reason = contracts.PlanChangeCancelled
		} else if !hadBefore {
			change.Reason = contracts.PlanChangeAssigned
		} else if !hasAfter {
			change.Reason = contracts.PlanChangeUnassigned
		} else if beforeAssignment.EngineerID != afterAssignment.EngineerID {
			change.Reason = contracts.PlanChangeReassigned
		} else {
			change.Reason = contracts.PlanChangeRescheduled
		}
		changes = append(changes, change)
	}
	return changes
}

func assignments(routes []contracts.Route) map[string]contracts.Assignment {
	result := make(map[string]contracts.Assignment)
	for _, route := range routes {
		for sequence, visit := range route.Visits {
			result[visit.OrderID] = contracts.Assignment{
				EngineerID: route.EngineerID,
				Sequence:   sequence,
				ArrivalAt:  visit.ArrivalAt,
				StartAt:    visit.StartAt,
				EndAt:      visit.EndAt,
			}
		}
	}
	return result
}

func assignmentEqual(left, right contracts.Assignment) bool {
	return left.EngineerID == right.EngineerID && left.Sequence == right.Sequence && left.ArrivalAt.Equal(right.ArrivalAt) && left.StartAt.Equal(right.StartAt) && left.EndAt.Equal(right.EndAt)
}
