package geo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestProviderFailuresPreserveCauseWithoutExposingDiagnostics(t *testing.T) {
	cause := errors.New(`Get "https://routing.openstreetmap.de/private": net/http: TLS handshake timeout`)
	points := []contracts.Location{
		{ID: "office", Point: contracts.Point{Lat: 55.7, Lon: 37.7}},
		{ID: "order", Point: contracts.Point{Lat: 55.8, Lon: 37.8}},
	}
	cases := []struct {
		name        string
		call        func() error
		detailKey   string
		detailValue any
	}{
		{"geocode", func() error {
			_, err := NewGeoService(NewFailingGeocodeProvider(cause)).Geocode(context.Background(), contracts.GeocodeRequest{
				Locations: []contracts.LocationInput{{ID: "order", Address: "Москва, дом 10"}},
			})
			return err
		}, "entity_id", "order"},
		{"matrix", func() error {
			_, err := NewGeoService(NewFailingRouteProvider(cause)).BuildMatrix(context.Background(), contracts.MatrixRequest{
				Locations: points, Profiles: []contracts.Transport{contracts.TransportCar},
			})
			return err
		}, "profile", contracts.TransportCar},
		{"geometry", func() error {
			_, err := NewGeoService(NewFailingRouteProvider(cause)).BuildRoutes(context.Background(), contracts.RoutesRequest{
				Locations: points, Legs: []contracts.RoutesRequestLeg{{LegID: "leg-1", FromLocationID: "office", ToLocationID: "order", Profile: contracts.TransportCar}},
			})
			return err
		}, "leg_id", "leg-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			var ce *contracts.ContractError
			if !errors.As(err, &ce) || ce.Code != "GEO_UNAVAILABLE" {
				t.Fatalf("expected GEO_UNAVAILABLE, got %v", err)
			}
			if !errors.Is(err, cause) {
				t.Fatal("underlying provider error must remain available to server callers")
			}
			if ce.Details[tc.detailKey] != tc.detailValue {
				t.Fatalf("missing useful context: %v", ce.Details)
			}
			encoded, err := json.Marshal(ce)
			if err != nil {
				t.Fatal(err)
			}
			for _, diagnostic := range []string{"https://", "TLS", "net/http", "geometry unavailable"} {
				if strings.Contains(string(encoded), diagnostic) {
					t.Fatalf("API error leaks provider diagnostic %q: %s", diagnostic, encoded)
				}
			}
			if !strings.Contains(ce.Message, "Повторите") {
				t.Fatalf("missing actionable error message: %s", ce.Message)
			}
		})
	}
}
