package plans

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestLateCompletionDoesNotConfirmNextPlannedTrip(t *testing.T) {
	for _, sameAddress := range []bool{true, false} {
		name := "next address"
		if sameAddress {
			name = "same address with zero travel"
		}
		t.Run(name, func(t *testing.T) {
			snapshot := testSnapshot()
			base := testSavedPlan(snapshot)
			next := snapshot.Orders[0]
			next.ID, next.SourceOrder = "order-2", 2
			travel := time.Duration(0)
			distance := int64(0)
			if !sameAddress {
				next.LocationID = "loc-2"
				snapshot.Locations = append(snapshot.Locations, contracts.Location{ID: next.LocationID, Point: contracts.Point{Lat: 55.77, Lon: 37.62}})
				travel, distance = 15*time.Minute, 1200
			}
			snapshot.Orders = append(snapshot.Orders, next)
			departure, started, expectedEnd := base.Routes[0].Legs[0].StartAt, base.Routes[0].Visits[0].StartAt, base.Routes[0].Visits[0].EndAt
			snapshot.Orders[0].Status = contracts.OrderStatusInProgress
			snapshot.Orders[0].Execution = &contracts.OrderExecution{EngineerID: "eng-1", DepartedAt: &departure, StartedAt: &started, ExpectedEndAt: &expectedEnd}
			arrival := expectedEnd.Add(travel)
			base.Routes[0].Legs = append(base.Routes[0].Legs, contracts.Leg{ID: "not-yet-driven", FromLocationID: "loc-1", ToLocationID: next.LocationID, StartAt: expectedEnd, EndAt: arrival, DistanceM: distance, GeoContextID: "geo-1", Geometry: []contracts.Point{snapshot.Locations[1].Point, snapshot.Locations[len(snapshot.Locations)-1].Point}})
			base.Routes[0].Visits = append(base.Routes[0].Visits, contracts.Visit{OrderID: next.ID, ArrivalAt: arrival, StartAt: arrival, EndAt: arrival.Add(30 * time.Minute)})
			// Another crew's event advanced the plan clock, but this crew has
			// not confirmed completion and could not depart on its next trip.
			base.AsOf = arrival
			finished := arrival.Add(15 * time.Minute)
			service := mustService(t, &fakeData{snapshot: snapshot, plan: base}, standardGeo(), &fakePlanner{solveFn: solveFirstOrder})
			result, err := service.Replan(context.Background(), contracts.ReplanRequest{
				RequestID: "finish-overdue", ScenarioID: snapshot.ScenarioID, SnapshotRevision: snapshot.Revision, BasePlanID: base.ID,
				Event: contracts.Event{ID: "finish-overdue", Type: contracts.EventOrderStatusChanged, OccurredAt: finished, Payload: contracts.EncodePayload(contracts.EventPayload{OrderID: "order-1", EngineerID: "eng-1", Status: contracts.OrderStatusCompleted})},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Draft.Routes) != 1 || len(result.Draft.Routes[0].Visits) != 2 {
				t.Fatalf("lost completed or next visit: %+v", result.Draft.Routes)
			}
			route := result.Draft.Routes[0]
			if !route.Visits[0].EndAt.Equal(finished) || !route.Visits[1].StartAt.Equal(finished.Add(travel)) {
				t.Fatalf("next visit must follow actual completion and travel: %+v", route.Visits)
			}
			if !reflect.DeepEqual(route.Legs[0], base.Routes[0].Legs[0]) {
				t.Fatal("already driven travel changed")
			}
			for _, leg := range route.Legs {
				if leg.ID == "not-yet-driven" {
					t.Fatal("a planned trip was recorded as driven before the preceding work finished")
				}
			}
		})
	}
}
