package plans

import (
	"context"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/contracts"
)

func TestRestoredAddressKeepsOriginalWorkAndRemovesRejection(t *testing.T) {
	snapshot := testSnapshot()
	base := testSavedPlan(snapshot)
	original := snapshot.Orders[0]
	original.ID = "restored"
	original.LocationID = "restored-location"
	original.WorkType = contracts.WorkTypeRepair
	original.SourceOrder = 12
	snapshot.UnlocatedOrders = []contracts.UnlocatedOrder{{Order: original, Address: "old", Message: "unknown"}}
	id := original.LocationID
	snapshot.Issues = append(snapshot.Issues, contracts.Issue{EntityID: &id, Code: "GEO_UNAVAILABLE", Message: "unknown"})
	geocoder := standardGeo()
	geocoder.geocodeFn = func(in contracts.GeocodeRequest) (contracts.GeocodeResult, error) {
		loc := in.Locations[0]
		return contracts.GeocodeResult{Items: []contracts.GeocodeItem{{LocationID: loc.ID, Location: &contracts.Location{ID: loc.ID, Address: loc.Address, Point: *loc.Point}}}}, nil
	}
	service := mustService(t, &fakeData{}, geocoder, &fakePlanner{})
	supplied := original
	supplied.RequiredSkills = []string{"changed"}
	supplied.ServiceSec = 60
	at := base.AsOf.Add(9 * time.Hour)
	supplied.ReceivedAt = at
	point := contracts.Point{Lat: 55.76, Lon: 37.615}
	target := cloneSnapshot(snapshot)
	_, _, err := service.applyEvent(context.Background(), &target, base, contracts.Event{ID: "resolve", Type: contracts.EventOrdinaryOrderAdded, OccurredAt: at, Payload: contracts.EncodePayload(contracts.EventPayload{Order: &supplied, Location: &contracts.LocationInput{ID: original.LocationID, Address: "Москва, Петровка, 2", Point: &point}})}, replayResult{})
	if err != nil {
		t.Fatal(err)
	}
	if len(target.UnlocatedOrders) != 0 || len(target.Orders) != 2 || len(target.Issues) != 0 {
		t.Fatalf("restored order duplicated or missing: %+v", target)
	}
	actual := target.Orders[1]
	if actual.ServiceSec != original.ServiceSec || actual.SourceOrder != original.SourceOrder || actual.RequiredSkills[0] != original.RequiredSkills[0] || actual.Window != original.Window {
		t.Fatalf("imported work changed: %+v", actual)
	}
	if len(snapshot.UnlocatedOrders) != 1 || len(snapshot.Orders) != 1 || len(snapshot.Issues) != 1 {
		t.Fatal("base snapshot mutated")
	}
}
