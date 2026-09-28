package plans

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/geo"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner"
)

func ordinaryFixture() (contracts.Snapshot, contracts.Plan) {
	s := testSnapshot()
	s.Orders[0].WorkType = contracts.WorkTypeRepair
	s.Orders[0].ReceivedAt = mustTime("2026-09-17T05:00:00Z")
	s.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{}
	s.Engineers[0].EquipmentStock = map[contracts.Equipment]int64{}
	return s, testSavedPlan(s)
}
func ordinaryReplan(t *testing.T, s contracts.Snapshot, p contracts.Plan, at time.Time, newLocation string, end time.Time) contracts.PlanResult {
	t.Helper()
	g := standardGeo()
	g.positionFn = func(input contracts.PositionRequest) (contracts.PositionResult, error) {
		return geo.NewGeoService(&geo.DemoProvider{}).PositionAt(context.Background(), input)
	}
	svc := mustService(t, &fakeData{snapshot: s, plan: p}, g, planner.New())
	fresh := contracts.Order{ID: "fresh", LocationID: newLocation, WorkType: contracts.WorkTypeRepair, RequiredSkills: []string{"repair"}, EquipmentRequired: map[contracts.Equipment]int64{}, Window: contracts.Window{Start: at, End: end}, ReceivedAt: at, ServiceSec: 600, Priority: contracts.PriorityNormal, Status: contracts.OrderStatusActive}
	result, err := svc.Replan(context.Background(), contracts.ReplanRequest{RequestID: "insert", ScenarioID: s.ScenarioID, SnapshotRevision: s.Revision, BasePlanID: p.ID, Event: contracts.Event{ID: "ordinary", OccurredAt: at, Type: contracts.EventOrdinaryOrderAdded, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &fresh})}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.TargetSnapshot.Orders) != len(s.Orders)+1 || result.AppliedEvent == nil {
		t.Fatal("new order/event not persisted")
	}
	for _, saved := range p.Routes {
		for _, old := range saved.Visits {
			found := 0
			for _, r := range result.Draft.Routes {
				for _, v := range r.Visits {
					if v.OrderID == old.OrderID {
						found++
						if v != old || r.EngineerID != saved.EngineerID {
							t.Fatal("old visit changed")
						}
					}
				}
			}
			if found != 1 {
				t.Fatalf("old visit occurs %d times", found)
			}
		}
	}
	return result
}
func TestOrdinaryReplanInsertsBeforeFirstVisit(t *testing.T) {
	s, p := ordinaryFixture()
	p.Routes[0].Legs[0].StartAt = mustTime("2026-09-17T06:45:00Z")
	p.Routes[0].Legs[0].EndAt = mustTime("2026-09-17T07:00:00Z")
	p.Routes[0].Visits[0].ArrivalAt = p.Routes[0].Legs[0].EndAt
	r := ordinaryReplan(t, s, p, mustTime("2026-09-17T06:00:00Z"), "office-1", mustTime("2026-09-17T06:30:00Z"))
	if len(r.Draft.Unassigned) != 0 || len(r.Draft.Routes[0].Visits) != 2 || r.Draft.Routes[0].Visits[0].OrderID != "fresh" {
		t.Fatalf("not inserted before fixed visit: %+v", r.Draft)
	}
}
func TestOrdinaryReplanNoSlotAndPreviousUnassigned(t *testing.T) {
	s, p := ordinaryFixture()
	previous := s.Orders[0]
	previous.ID = "previous-unassigned"
	previous.SourceOrder = 2
	s.Orders = append(s.Orders, previous)
	old := contracts.UnassignedOrder{OrderID: previous.ID, ReasonCode: "NOT_ASSIGNED_BY_SOLVER", Message: "previous"}
	p.Unassigned = append(p.Unassigned, old)
	r := ordinaryReplan(t, s, p, mustTime("2026-09-17T06:00:00Z"), "loc-1", mustTime("2026-09-17T06:01:00Z"))
	if len(r.Draft.Unassigned) != 2 || r.Draft.Unassigned[0] != old || r.Draft.Unassigned[1].ReasonCode != "NO_FEASIBLE_INSERTION" {
		t.Fatalf("unassigned=%+v", r.Draft.Unassigned)
	}
	if !reflect.DeepEqual(p.Routes, r.Draft.Routes) {
		t.Fatal("no-slot changed routes")
	}
}
func TestOrdinaryReplanMissedVisitIsSavedAsConflict(t *testing.T) {
	s, p := ordinaryFixture()
	r := ordinaryReplan(t, s, p, mustTime("2026-09-17T07:31:00Z"), "loc-1", mustTime("2026-09-17T09:00:00Z"))
	if len(r.Draft.Unassigned) != 1 || r.Draft.Unassigned[0].ReasonCode != "NOT_ASSIGNED_BY_SOLVER" {
		t.Fatal(r.Draft.Unassigned)
	}
	found := false
	for _, issue := range r.Draft.Issues {
		found = found || issue.Code == "EXISTING_PLAN_CONFLICT"
	}
	if !found {
		t.Fatal("missing conflict issue")
	}
	if !reflect.DeepEqual(p.Routes, r.Draft.Routes) {
		t.Fatal("conflict changed routes")
	}
}
func TestOrdinaryReplanPreservesCompletedHistory(t *testing.T) {
	s, p := ordinaryFixture()
	old := p.Routes[0].Visits[0]
	departed := p.Routes[0].Legs[0].StartAt
	s.Orders[0].Status = contracts.OrderStatusCompleted
	s.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departed, StartedAt: &old.StartAt, FinishedAt: &old.EndAt}
	next := s.Orders[0]
	next.ID = "next"
	next.Status = contracts.OrderStatusActive
	next.Execution = nil
	next.SourceOrder = 2
	s.Orders = append(s.Orders, next)
	p.Routes[0].Visits = append(p.Routes[0].Visits, contracts.Visit{OrderID: next.ID, ArrivalAt: mustTime("2026-09-17T08:00:00Z"), StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:30:00Z")})
	p.Routes[0].Legs = append(p.Routes[0].Legs, contracts.Leg{ID: "leg-next", FromLocationID: "loc-1", ToLocationID: "loc-1", StartAt: mustTime("2026-09-17T08:00:00Z"), EndAt: mustTime("2026-09-17T08:00:00Z"), GeoContextID: "geo-1", Geometry: []contracts.Point{s.Locations[1].Point, s.Locations[1].Point}})
	r := ordinaryReplan(t, s, p, mustTime("2026-09-17T07:31:00Z"), "loc-1", mustTime("2026-09-17T07:45:00Z"))
	if len(r.Draft.Unassigned) != 0 || r.Draft.Metrics.CompletedCount != 1 || len(r.Draft.Routes[0].Visits) != 3 {
		t.Fatalf("bad history/insertion: %+v", r.Draft)
	}
	if !reflect.DeepEqual(r.Draft.Routes[0].Legs[0], p.Routes[0].Legs[0]) {
		t.Fatal("actual trip changed")
	}
}
func TestOrdinaryReplanProtectsCurrentTrip(t *testing.T) {
	s, p := ordinaryFixture()
	departure := p.Routes[0].Legs[0].StartAt
	s.Orders[0].Status = contracts.OrderStatusEnRoute
	s.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure}
	r := ordinaryReplan(t, s, p, mustTime("2026-09-17T06:07:30Z"), "loc-1", mustTime("2026-09-17T09:00:00Z"))
	if len(r.Draft.Unassigned) != 0 || len(r.Draft.Routes[0].Visits) != 2 || r.Draft.Routes[0].Visits[0].OrderID != "order-1" {
		t.Fatal("current trip redirected or insertion lost")
	}
	legs := r.Draft.Routes[0].Legs
	if len(legs) < 2 || legs[0].DistanceM+legs[1].DistanceM != p.Routes[0].Legs[0].DistanceM || !legs[0].EndAt.Equal(legs[1].StartAt) || !legs[1].EndAt.Equal(p.Routes[0].Legs[0].EndAt) {
		t.Fatalf("elapsed and remaining travel were not preserved: %+v", legs)
	}
}

func TestOrdinaryFallbackKeepsFutureVisitsAfterCurrentTrip(t *testing.T) {
	for _, tc := range []struct {
		name      string
		at        string
		available bool
	}{
		{"unavailable_crew", "2026-09-17T06:07:30Z", false},
		{"overdue_unconfirmed_start", "2026-09-17T07:31:00Z", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := ordinaryFixture()
			s.Engineers[0].Available = tc.available
			p.AsOf = mustTime("2026-09-17T06:07:00Z")
			departure := p.Routes[0].Legs[0].StartAt
			s.Orders[0].Status = contracts.OrderStatusEnRoute
			s.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure}
			next := s.Orders[0]
			next.ID, next.SourceOrder = "next", 2
			next.Status, next.Execution = contracts.OrderStatusActive, nil
			s.Orders = append(s.Orders, next)
			start := mustTime("2026-09-17T08:00:00Z")
			p.Routes[0].Visits = append(p.Routes[0].Visits, contracts.Visit{OrderID: next.ID, ArrivalAt: start, StartAt: start, EndAt: start.Add(30 * time.Minute)})
			p.Routes[0].Legs = append(p.Routes[0].Legs, contracts.Leg{ID: "leg-next", FromLocationID: "loc-1", ToLocationID: "loc-1", StartAt: start, EndAt: start, GeoContextID: "geo-1", Geometry: []contracts.Point{s.Locations[1].Point, s.Locations[1].Point}})

			r := ordinaryReplan(t, s, p, mustTime(tc.at), "loc-1", mustTime("2026-09-17T09:00:00Z"))
			if !reflect.DeepEqual(r.Draft.Routes, p.Routes) {
				t.Fatal("failed insertion changed the accepted schedule")
			}
			if len(r.Draft.Unassigned) != 1 || r.Draft.Unassigned[0].OrderID != "fresh" || r.Draft.Unassigned[0].ReasonCode != contracts.UnassignedBySolver {
				t.Fatalf("new work must remain unassigned with an explanation: %+v", r.Draft.Unassigned)
			}
		})
	}
}
