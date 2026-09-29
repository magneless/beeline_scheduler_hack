package plans

import (
	"context"
	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"sort"
	"time"
)

const pendingBatch = "pending_changes"

func eventList(event contracts.Event) []contracts.Event {
	if event.Type == pendingBatch {
		return contracts.DecodePayload(event.Payload).Events
	}
	return []contracts.Event{event}
}

func eventIncludes(event contracts.Event, kind string) bool {
	for _, item := range eventList(event) {
		if item.Type == kind {
			return true
		}
	}
	return false
}

func ordinaryOnly(event contracts.Event) bool {
	items := eventList(event)
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Type != contracts.EventOrdinaryOrderAdded {
			return false
		}
	}
	return true
}

// A queue that contains only facts with no routing consequences still uses
// the normal validation, but never moves the accepted appointments.
func (s *Service) recordPendingFacts(ctx context.Context, snapshot contracts.Snapshot, base contracts.Plan, event contracts.Event, mode contracts.SolveMode) (contracts.PlanResult, bool, error) {
	if event.Type != pendingBatch {
		return contracts.PlanResult{}, false, nil
	}
	items, err := orderedEvents(snapshot, base, event)
	if err != nil {
		return contracts.PlanResult{}, true, err
	}
	target, working := cloneSnapshot(snapshot), base
	var last contracts.PlanResult
	for _, item := range items {
		var result contracts.PlanResult
		var handled bool
		var err error
		switch item.Type {
		case contracts.EventOrderStatusChanged:
			result, handled, err = s.recordWorkStatus(ctx, target, working, item, mode)
		case contracts.EventOrderCancelled:
			result, handled, err = s.recordUnassignedCancellation(ctx, target, working, item)
		default:
			return contracts.PlanResult{}, false, nil
		}
		if !handled {
			return contracts.PlanResult{}, false, nil
		}
		if err != nil {
			return result, true, err
		}
		last, target = result, result.TargetSnapshot
		working = contracts.Plan{ID: base.ID, PlanDraft: result.Draft}
	}
	if len(eventList(event)) == 0 {
		return contracts.PlanResult{}, false, nil
	}
	last.TargetSnapshot.Revision, last.Draft.SnapshotRevision = snapshot.Revision+1, snapshot.Revision+1
	last.Draft.BasePlanID, last.AppliedEvent = &base.ID, &event
	last.Draft.Changes = calculateChanges(base.Routes, last.Draft.Routes, nil)
	return last, true, nil
}

// PreparePending validates and normalizes the whole queue without calculating
// routes. All events describe the accepted plan, never an intermediate solution.
func (s *Service) PreparePending(ctx context.Context, input contracts.ReplanRequest) (contracts.Snapshot, contracts.Event, error) {
	snapshot, err := s.data.GetSnapshot(ctx, input.ScenarioID, input.SnapshotRevision)
	if err != nil {
		return snapshot, contracts.Event{}, err
	}
	base, err := s.data.GetPlan(ctx, input.BasePlanID)
	if err != nil {
		return snapshot, contracts.Event{}, err
	}
	if base.ScenarioID != snapshot.ScenarioID || base.SnapshotRevision != snapshot.Revision {
		return snapshot, contracts.Event{}, contracts.EventConflict("План изменился. Обновите смену.", nil)
	}
	if workFactsOnly(input.Event) {
		items, err := orderedEvents(snapshot, base, input.Event)
		if err != nil {
			return snapshot, contracts.Event{}, err
		}
		target := cloneSnapshot(snapshot)
		for _, item := range items {
			if _, _, err = s.applyEvent(ctx, &target, base, item, replayResult{}); err != nil {
				return target, input.Event, err
			}
		}
		err = validateSnapshot(target, target.ScenarioID, target.Revision)
		return target, input.Event, err
	}
	target, _, applied, _, err := s.prepareEvents(ctx, snapshot, base, input.Event)
	return target, applied, err
}

func (s *Service) prepareEvents(ctx context.Context, snapshot contracts.Snapshot, base contracts.Plan, event contracts.Event) (contracts.Snapshot, replayResult, contracts.Event, []string, error) {
	target := cloneSnapshot(snapshot)
	var replay replayResult
	var cancelled []string
	items, err := orderedEvents(snapshot, base, event)
	if err != nil {
		return target, replay, event, nil, err
	}
	stoppedRoutes := map[string][]contracts.Route{}
	normalized := make([]contracts.Event, 0, len(items))
	for _, item := range items {
		if !workFactsOnly(item) {
			replay, err = s.replayAt(ctx, &target, base, item)
			if err != nil {
				return target, replay, event, nil, err
			}
		}
		payload := contracts.DecodePayload(item.Payload)
		if item.Type == contracts.EventOrderCancelled && len(payload.OrderIDs) > 0 {
			for _, id := range payload.OrderIDs {
				part := item
				part.Payload = contracts.EncodePayload(contracts.EventPayload{OrderID: id, Reason: payload.Reason})
				_, removed, e := s.applyEvent(ctx, &target, base, part, replay)
				if e != nil {
					return target, replay, event, nil, e
				}
				cancelled = append(cancelled, removed...)
			}
			normalized = append(normalized, item)
		} else {
			applied, removed, e := s.applyEvent(ctx, &target, base, item, replay)
			if e != nil {
				return target, replay, event, nil, e
			}
			cancelled = append(cancelled, removed...)
			normalized = append(normalized, applied)
		}
		if item.Type == contracts.EventEngineerUnavailable {
			stoppedRoutes[payload.EngineerID] = nil
			for _, route := range replay.routes {
				if route.EngineerID == payload.EngineerID {
					stoppedRoutes[payload.EngineerID] = clonePlanRoutes([]contracts.Route{route})
				}
			}
		}
	}
	if event.Type == pendingBatch {
		// Preserve entry order for Undo; calculations apply a chronological copy.
		byID := map[string]contracts.Event{}
		for _, item := range normalized {
			byID[item.ID] = item
		}
		entered := eventList(event)
		for i := range entered {
			entered[i] = byID[entered[i].ID]
		}
		event.Payload = contracts.EncodePayload(contracts.EventPayload{Events: entered})
	} else {
		event = normalized[0]
	}
	calculationEvent := event
	calculationEvent.OccurredAt = laterTime(base.AsOf, event.OccurredAt)
	if event.Type == pendingBatch || workFactsOnly(event) {
		replay, err = s.replayAt(ctx, &target, base, calculationEvent)
		if err != nil {
			return target, replay, event, nil, err
		}
	}
	// Later queued events cannot make an unavailable crew keep travelling.
	for engineerID, frozen := range stoppedRoutes {
		kept := replay.routes[:0]
		for _, route := range replay.routes {
			if route.EngineerID != engineerID {
				kept = append(kept, route)
			}
		}
		orders := orderMap(target.Orders)
		for i := range frozen {
			for j, visit := range frozen[i].Visits {
				frozen[i].Visits[j] = factualVisit(orders[visit.OrderID], visit, calculationEvent.OccurredAt)
			}
		}
		replay.routes = append(kept, frozen...)
	}
	if err = refreshExecution(&target, &replay, calculationEvent); err != nil {
		return target, replay, event, nil, err
	}
	if err = validateSnapshot(target, target.ScenarioID, target.Revision); err != nil {
		return target, replay, event, nil, err
	}
	return target, replay, event, cancelled, nil
}

func laterTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func workFactsOnly(event contracts.Event) bool {
	items := eventList(event)
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		payload := contracts.DecodePayload(item.Payload)
		if item.Type != contracts.EventOrderStatusChanged || (payload.Status != contracts.OrderStatusInProgress && payload.Status != contracts.OrderStatusCompleted) {
			return false
		}
	}
	return true
}

func orderedEvents(snapshot contracts.Snapshot, base contracts.Plan, event contracts.Event) ([]contracts.Event, error) {
	items := append([]contracts.Event(nil), eventList(event)...)
	if len(items) == 0 || len(items) > 100 {
		return nil, contracts.InvalidInput("Очередь должна содержать от 1 до 100 событий", nil)
	}
	dayStart, dayEnd, err := localDayBounds(snapshot)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	latest := items[0].OccurredAt
	for _, item := range items {
		if item.ID == "" || seen[item.ID] || item.Type == pendingBatch || item.OccurredAt.Before(dayStart) || !item.OccurredAt.Before(dayEnd) || (item.OccurredAt.Before(base.AsOf) && !workFactsOnly(item)) {
			return nil, contracts.InvalidInput("Проверьте время событий: факты работ должны относиться к дню сценария, остальные изменения — не предшествовать принятому плану", nil)
		}
		seen[item.ID] = true
		latest = laterTime(latest, item.OccurredAt)
	}
	if !event.OccurredAt.Equal(latest) {
		return nil, contracts.InvalidInput("Время очереди не совпадает с последним событием", nil)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items, nil
}
