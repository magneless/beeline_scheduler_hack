package plans

import (
	"context"
	"encoding/json"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

// An unassigned order occupies no crew's time or equipment. Cancelling it must
// not replay other crews or replace their accepted routes with new predictions.
func (service *Service) recordUnassignedCancellation(ctx context.Context, snapshot contracts.Snapshot, base contracts.Plan, event contracts.Event) (contracts.PlanResult, bool, error) {
	if event.Type != contracts.EventOrderCancelled {
		return contracts.PlanResult{}, false, nil
	}
	var payload contracts.OrderCancelled
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return contracts.PlanResult{}, true, contracts.InvalidInput("invalid cancellation payload", nil)
	}
	ids, err := payload.IDs()
	if err != nil {
		return contracts.PlanResult{}, true, err
	}
	unassigned := make(map[string]bool, len(base.Unassigned))
	for _, item := range base.Unassigned {
		unassigned[item.OrderID] = true
	}
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if !unassigned[id] {
			if payload.OrderIDs != nil {
				return contracts.PlanResult{}, true, contracts.EventConflict("Для общей отмены выберите только неназначенные заявки", map[string]any{"order_id": id})
			}
			return contracts.PlanResult{}, false, nil
		}
		selected[id] = struct{}{}
	}
	progress.Report(ctx, "saving", "Сохраняем отмену заявок")
	target := cloneSnapshot(snapshot)
	for _, id := range ids {
		itemEvent := event
		itemEvent.Payload = contracts.EncodePayload(contracts.EventPayload{OrderID: id, Reason: payload.Reason})
		if _, _, err := service.applyEvent(ctx, &target, base, itemEvent, replayResult{}); err != nil {
			return contracts.PlanResult{}, true, err
		}
	}
	target.Revision++
	if err := validateSnapshot(target, target.ScenarioID, target.Revision); err != nil {
		return contracts.PlanResult{}, true, err
	}
	draft := base.PlanDraft
	draft.OptionKey = "strict"
	draft.ReserveEngineerIDs = nil
	draft.BaselineMetrics = nil
	draft.SnapshotRevision = target.Revision
	draft.BasePlanID = &base.ID
	draft.AsOf = event.OccurredAt
	draft.Routes = clonePlanRoutes(base.Routes)
	draft.Unassigned = []contracts.UnassignedOrder{}
	for _, item := range base.Unassigned {
		if _, cancelled := selected[item.OrderID]; !cancelled {
			draft.Unassigned = append(draft.Unassigned, item)
		}
	}
	draft.DeferredOrderIDs = nil
	for _, id := range base.DeferredOrderIDs {
		if _, cancelled := selected[id]; !cancelled {
			draft.DeferredOrderIDs = append(draft.DeferredOrderIDs, id)
		}
	}
	draft.CancelledOrderIDs = mergeCancelled(base.CancelledOrderIDs, ids)
	draft.Metrics.UnassignedCount = len(draft.Unassigned)
	draft.Changes = calculateChanges(base.Routes, draft.Routes, selected)
	draft.Termination = contracts.TerminationCompleted
	if err := validateFinalPlan(target, draft.Routes, draft.Unassigned, draft.CancelledOrderIDs, draft.Lateness); err != nil {
		return contracts.PlanResult{}, true, err
	}
	return contracts.PlanResult{Draft: draft, TargetSnapshot: target, AppliedEvent: &event}, true, nil
}
