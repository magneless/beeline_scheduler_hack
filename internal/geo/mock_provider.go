package geo

import (
	"context"
	"fmt"
	"math"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

// MockRouteProvider is a configurable test double for RouteProvider.
//
// By default it returns reasonable fake data. Override GeocodeFunc / RouteFunc
// to control behavior per-test.
type MockRouteProvider struct {
	GeocodeFunc func(ctx context.Context, address string) ([]contracts.Location, error)
	RouteFunc   func(ctx context.Context, from, to contracts.Point, transport contracts.Transport) (RouteResult, error)
}

// Geocode delegates to GeocodeFunc if set; otherwise returns a single candidate.
func (m *MockRouteProvider) Geocode(ctx context.Context, address string) ([]contracts.Location, error) {
	if m.GeocodeFunc != nil {
		return m.GeocodeFunc(ctx, address)
	}
	return []contracts.Location{
		{
			ID:      "resolved-1",
			Address: address,
			Point:   contracts.Point{Lat: 55.7558, Lon: 37.6173},
		},
	}, nil
}

// Route delegates to RouteFunc if set; otherwise returns a straight-line route.
func (m *MockRouteProvider) Route(ctx context.Context, from, to contracts.Point, transport contracts.Transport) (RouteResult, error) {
	if m.RouteFunc != nil {
		return m.RouteFunc(ctx, from, to, transport)
	}

	mid := contracts.Point{
		Lat: (from.Lat + to.Lat) / 2,
		Lon: (from.Lon + to.Lon) / 2,
	}
	dist := haversineDistance(from, to)
	distM := int64(math.Round(dist))

	// Assume ~50 km/h for car, ~5 km/h for walk.
	speed := 50_000.0 / 3600.0
	if transport == contracts.TransportWalk {
		speed = 5_000.0 / 3600.0
	}
	durSec := int64(math.Round(dist / speed))
	if durSec == 0 && distM > 0 {
		durSec = 1
	}

	return RouteResult{
		Reachable:   true,
		DurationSec: durSec,
		DistanceM:   distM,
		Geometry:    []contracts.Point{from, mid, to},
	}, nil
}

// --- Convenience constructors for common test scenarios ---

// NewFailingGeocodeProvider returns a provider whose Geocode always fails.
func NewFailingGeocodeProvider(err error) *MockRouteProvider {
	return &MockRouteProvider{
		GeocodeFunc: func(_ context.Context, _ string) ([]contracts.Location, error) {
			return nil, err
		},
	}
}

// NewAmbiguousGeocodeProvider returns a provider that always returns
// multiple candidates (simulating an ambiguous address).
func NewAmbiguousGeocodeProvider() *MockRouteProvider {
	return &MockRouteProvider{
		GeocodeFunc: func(_ context.Context, address string) ([]contracts.Location, error) {
			return []contracts.Location{
				{ID: "cand-a", Address: address + " (вариант А)", Point: contracts.Point{Lat: 55.75, Lon: 37.61}},
				{ID: "cand-b", Address: address + " (вариант Б)", Point: contracts.Point{Lat: 55.76, Lon: 37.62}},
			}, nil
		},
	}
}

// NewEmptyGeocodeProvider returns a provider that always returns 0 candidates.
func NewEmptyGeocodeProvider() *MockRouteProvider {
	return &MockRouteProvider{
		GeocodeFunc: func(_ context.Context, _ string) ([]contracts.Location, error) {
			return []contracts.Location{}, nil
		},
	}
}

// NewFailingRouteProvider returns a provider whose Route always fails.
func NewFailingRouteProvider(err error) *MockRouteProvider {
	return &MockRouteProvider{
		RouteFunc: func(_ context.Context, _, _ contracts.Point, _ contracts.Transport) (RouteResult, error) {
			return RouteResult{}, err
		},
	}
}

// NewUnreachableRouteProvider returns a provider whose Route always returns unreachable.
func NewUnreachableRouteProvider() *MockRouteProvider {
	return &MockRouteProvider{
		RouteFunc: func(_ context.Context, _, _ contracts.Point, _ contracts.Transport) (RouteResult, error) {
			return RouteResult{Reachable: false}, nil
		},
	}
}

// NewSelectiveRouteProvider returns a provider that fails at a specific call index.
func NewSelectiveRouteProvider(failAtCall int) *MockRouteProvider {
	callCount := 0
	base := &MockRouteProvider{}
	base.RouteFunc = func(ctx context.Context, from, to contracts.Point, transport contracts.Transport) (RouteResult, error) {
		current := callCount
		callCount++
		if current == failAtCall {
			return RouteResult{}, fmt.Errorf("route unavailable for call %d", current)
		}
		tmp := &MockRouteProvider{}
		return tmp.Route(ctx, from, to, transport)
	}
	return base
}
