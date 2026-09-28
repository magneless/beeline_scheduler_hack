package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

type optionFixtureService struct{ *testkit.Plans }

func (s *optionFixtureService) BuildOptions(ctx context.Context, in c.BuildPlanRequest) ([]c.PlanOption, error) {
	result, err := s.Build(ctx, in)
	if err != nil {
		return nil, err
	}
	result.Draft.OptionKey = "strict"
	return []c.PlanOption{{Key: "strict", Label: "В окнах", Result: result}}, nil
}

func (s *optionFixtureService) ReplanOptions(ctx context.Context, in c.ReplanRequest) ([]c.PlanOption, error) {
	result, err := s.Replan(ctx, in)
	if err != nil {
		return nil, err
	}
	result.Draft.OptionKey = "strict"
	return []c.PlanOption{{Key: "strict", Label: "Текущий состав", Result: result}}, nil
}

func TestHTTPProposalsPersistUntilAccepted(t *testing.T) {
	_, store, fixture := integrationServer(t)
	ctx := context.Background()
	snap := fixture.Snapshot
	snap.ScenarioID = storage.ID("scenario")
	view, err := store.CreateScenario(ctx, snap, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	service := &optionFixtureService{Plans: &testkit.Plans{Reader: store, Fixture: fixture}}
	h := (&Server{Store: store, Plans: service}).Handler()
	sid := view.Snapshot.ScenarioID
	var proposal storage.Proposal
	request := map[string]any{"request_id": "preview-1", "snapshot_revision": 1, "expected_current_plan_id": nil}
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", request, 201, &proposal)
	if proposal.ID == "" || len(proposal.Options) != 1 || proposal.Options[0].Key != "strict" {
		t.Fatal(proposal)
	}
	var pending storage.Proposal
	call(t, h, "GET", "/scenarios/"+sid+"/proposals/current", nil, 200, &pending)
	if pending.ID != proposal.ID {
		t.Fatal("pending proposal did not survive a new handler")
	}
	call(t, h, "POST", "/scenarios/"+sid+"/plans", request, 409, nil)
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-1", "option_key": "unknown"}, 422, nil)
	var plan c.Plan
	accept := map[string]any{"request_id": "accept-1", "option_key": "strict"}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", accept, 200, &plan)
	if plan.ID == "" || plan.OptionKey != "strict" {
		t.Fatal(plan)
	}
	var repeat c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", accept, 200, &repeat)
	if repeat.ID != plan.ID {
		t.Fatal("acceptance was not idempotent")
	}
	view, err = store.GetScenario(ctx, sid, 0)
	if err != nil || view.CurrentPlanID == nil || *view.CurrentPlanID != plan.ID || view.Snapshot.Revision != 1 {
		t.Fatalf("unexpected scenario after acceptance: %+v, %v", view, err)
	}
	event := fixture.ReplanRequest.Event
	request = map[string]any{"request_id": "preview-event", "snapshot_revision": 1, "expected_current_plan_id": plan.ID, "event": event}
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", request, 201, &proposal)
	view, err = store.GetScenario(ctx, sid, 0)
	if err != nil || view.Snapshot.Revision != 1 || *view.CurrentPlanID != plan.ID {
		t.Fatal("event preview changed accepted scenario")
	}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-event", "option_key": "strict"}, 200, &plan)
	view, err = store.GetScenario(ctx, sid, 0)
	if err != nil || view.Snapshot.Revision != 2 || *view.CurrentPlanID != plan.ID {
		t.Fatalf("event acceptance did not commit: %+v, %v", view, err)
	}
}

func TestRealPlannerProposalBuildAndEvent(t *testing.T) {
	h, _, fixture := integrationServer(t, true, true)
	var view c.ScenarioView
	call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": "contract-example"}, 201, &view)
	sid := view.Snapshot.ScenarioID
	var proposal storage.Proposal
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{
		"request_id": "build-options", "snapshot_revision": 1, "expected_current_plan_id": nil,
	}, 201, &proposal)
	if len(proposal.Options) != 2 || proposal.Options[0].Key != "strict" || proposal.Options[1].Key != "late_emergency" {
		t.Fatalf("initial variants: %+v", proposal.Options)
	}
	var plan c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-build", "option_key": "strict"}, 200, &plan)
	if plan.ID == "" || plan.OptionKey != "strict" {
		t.Fatal(plan)
	}
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{
		"request_id": "event-options", "snapshot_revision": 1, "expected_current_plan_id": plan.ID,
		"event": fixture.ReplanRequest.Event,
	}, 201, &proposal)
	if len(proposal.Options) != 3 || proposal.Options[0].Key != "strict" || proposal.Options[1].Key != "original" || proposal.Options[2].Key != "reserve" {
		t.Fatalf("event variants: %+v", proposal.Options)
	}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "accept-event", "option_key": "original"}, 200, &plan)
	if plan.OptionKey != "original" || plan.SnapshotRevision != 2 {
		t.Fatal(plan)
	}
}

func TestReserveActivationOnlyAfterSelectingReserveOption(t *testing.T) {
	h, store, fixture := integrationServer(t, true, true)
	snapshot := fixture.Snapshot
	snapshot.ScenarioID = storage.ID("scenario")
	reserve := snapshot.Engineers[0]
	reserve.ID = "reserve-used"
	reserve.SourceOrder = 2
	reserve.Skills = []string{"emergency", "special"}
	reserve.Available = true
	reserve.Reserve = false
	unused := reserve
	unused.ID = "reserve-unused"
	unused.SourceOrder = 3
	unused.Skills = []string{"emergency"}
	snapshot.Engineers = append(snapshot.Engineers, reserve, unused)
	view, err := store.CreateScenario(context.Background(), snapshot, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	sid := view.Snapshot.ScenarioID
	var proposal storage.Proposal
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{
		"request_id": "reserve-build", "snapshot_revision": 1, "expected_current_plan_id": nil,
	}, 201, &proposal)
	var plan c.Plan
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "reserve-accept-build", "option_key": "strict"}, 200, &plan)
	initialRevision := plan.SnapshotRevision
	order := snapshot.Orders[0]
	order.ID = "reserve-emergency"
	order.RequiredSkills = []string{"special"}
	order.EquipmentRequired = map[c.Equipment]int64{}
	order.WorkType = c.WorkTypeEmergency
	order.Priority = c.PriorityUrgent
	order.ServiceSec = 4800
	order.ReceivedAt = plan.AsOf
	order.Window.Start = plan.AsOf.Add(9 * time.Hour)
	order.Window.End = plan.AsOf.Add(12 * time.Hour)
	payload, err := json.Marshal(c.UrgentOrderAdded{Order: order})
	if err != nil {
		t.Fatal(err)
	}
	event := c.Event{ID: "reserve-event", Type: "urgent_order_added", OccurredAt: plan.AsOf, Payload: payload}
	call(t, h, "POST", "/scenarios/"+sid+"/proposals", map[string]any{
		"request_id": "reserve-preview", "snapshot_revision": initialRevision, "expected_current_plan_id": plan.ID, "event": event,
	}, 201, &proposal)
	if len(proposal.Options) != 3 {
		t.Fatalf("expected 3 emergency options, got %d", len(proposal.Options))
	}
	var reserveOption *c.PlanOption
	for i := range proposal.Options {
		if proposal.Options[i].Key == "reserve" {
			reserveOption = &proposal.Options[i]
		}
	}
	if reserveOption == nil || len(reserveOption.ReserveEngineerIDs) != 1 || reserveOption.ReserveEngineerIDs[0] != reserve.ID {
		t.Fatalf("unexpected selected reserve engineers: %+v", reserveOption)
	}
	view, err = store.GetScenario(context.Background(), sid, 0)
	if err != nil || view.Snapshot.Revision != initialRevision || view.Snapshot.Engineers[1].Available || view.Snapshot.Engineers[2].Available {
		t.Fatal("preview activated reserve engineers")
	}
	call(t, h, "POST", "/proposals/"+proposal.ID+"/accept", map[string]any{"request_id": "reserve-accept-event", "option_key": "reserve"}, 200, &plan)
	view, err = store.GetScenario(context.Background(), sid, 0)
	if err != nil || view.Snapshot.Revision != initialRevision+1 || !view.Snapshot.Engineers[1].Available || view.Snapshot.Engineers[1].Reserve || view.Snapshot.Engineers[2].Available || !view.Snapshot.Engineers[2].Reserve {
		t.Fatalf("accepted reserve state: %+v, %v", view.Snapshot.Engineers, err)
	}
}
