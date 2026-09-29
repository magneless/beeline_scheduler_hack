package geo

import (
	"context"
	"errors"
	"testing"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

func TestGeocodeProgressCountsProcessedAddresses(t *testing.T) {
	var updates []progress.Update
	ctx := progress.WithReporter(context.Background(), func(update progress.Update) { updates = append(updates, update) })
	provider := &MockRouteProvider{GeocodeFunc: func(_ context.Context, address string) ([]contracts.Location, error) {
		// No address is counted as complete until the provider has answered.
		want := 2
		if address == "ambiguous" {
			want = 3
		}
		if got := updates[len(updates)-1].Completed; got != want {
			t.Fatalf("provider called with %d completed addresses, want %d", got, want)
		}
		if address == "missing" {
			return nil, nil
		}
		return []contracts.Location{{Point: contracts.Point{Lat: 55, Lon: 37}}, {Point: contracts.Point{Lat: 56, Lon: 37}}}, nil
	}}
	result, err := NewGeoService(provider).Geocode(ctx, contracts.GeocodeRequest{Locations: []contracts.LocationInput{
		{ID: "known", Point: &contracts.Point{Lat: 55, Lon: 37}},
		{ID: "invalid", Point: &contracts.Point{Lat: 100, Lon: 37}},
		{ID: "missing", Address: "missing"},
		{ID: "ambiguous", Address: "ambiguous"},
	}})
	if err != nil || len(result.Items) != 4 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(updates) != 5 {
		t.Fatalf("expected initial progress and four completed addresses: %+v", updates)
	}
	for i, update := range updates {
		if update.Completed != i || update.Total != 4 {
			t.Fatalf("progress %d: %+v", i, update)
		}
	}
	if result.Items[0].Location == nil || result.Items[1].Issue == nil || result.Items[2].Issue == nil || result.Items[3].Issue == nil {
		t.Fatalf("progress changed geocoding results: %+v", result.Items)
	}
}

func TestGeocodeCancellationDoesNotReportCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var updates []progress.Update
	ctx = progress.WithReporter(ctx, func(update progress.Update) { updates = append(updates, update) })
	provider := &MockRouteProvider{GeocodeFunc: func(ctx context.Context, _ string) ([]contracts.Location, error) {
		cancel()
		return nil, ctx.Err()
	}}
	_, err := NewGeoService(provider).Geocode(ctx, contracts.GeocodeRequest{Locations: []contracts.LocationInput{{ID: "a", Address: "address"}}})
	if !errors.Is(err, context.Canceled) || len(updates) != 1 || updates[0].Completed != 0 || updates[0].Total != 1 {
		t.Fatalf("cancelled address reported as complete: err=%v progress=%+v", err, updates)
	}
}
