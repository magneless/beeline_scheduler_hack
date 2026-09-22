package geo

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
)

// ============================================================================
// Helpers
// ============================================================================

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// newSimpleLeg creates a leg along a meridian with an evenly-spaced 3-point
// polyline, making distances deterministic.
func newSimpleLeg() contracts.Leg {
	p0 := contracts.Point{Lat: 0, Lon: 0}
	p1 := contracts.Point{Lat: 1, Lon: 0}
	p2 := contracts.Point{Lat: 2, Lon: 0}

	totalDist := haversineDistance(p0, p1) + haversineDistance(p1, p2)

	return contracts.Leg{
		ID:             "test-leg",
		FromLocationID: "from",
		ToLocationID:   "to",
		StartAt:        mustTime("2026-09-17T06:00:00Z"),
		EndAt:          mustTime("2026-09-17T06:10:00Z"), // 600 sec
		DistanceM:      int64(math.Round(totalDist)),
		GeoContextID:   "geo-test",
		Geometry:       []contracts.Point{p0, p1, p2},
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

func assertContractError(t *testing.T, err error, expectedCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected ContractError with code %q, got nil", expectedCode)
	}
	var ce *contracts.ContractError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ContractError, got %T: %v", err, err)
	}
	if ce.Code != expectedCode {
		t.Fatalf("expected code %q, got %q (message: %s)", expectedCode, ce.Code, ce.Message)
	}
}

func assertInDelta(t *testing.T, expected, actual, delta float64, msg string) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Fatalf("%s: expected %f ±%f, got %f", msg, expected, delta, actual)
	}
}

func assertPointNear(t *testing.T, expected, actual contracts.Point, delta float64, msg string) {
	t.Helper()
	assertInDelta(t, expected.Lat, actual.Lat, delta, msg+" lat")
	assertInDelta(t, expected.Lon, actual.Lon, delta, msg+" lon")
}

// ============================================================================
// PositionAt Tests
// ============================================================================

func TestPositionAt_Start(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt,
	})
	assertNoError(t, err)

	assertPointNear(t, leg.Geometry[0], res.Point, 0.001, "start point")
	if res.ElapsedDurationSec != 0 {
		t.Errorf("expected elapsed_duration_sec=0, got %d", res.ElapsedDurationSec)
	}
	if res.ElapsedDistanceM != 0 {
		t.Errorf("expected elapsed_distance_m=0, got %d", res.ElapsedDistanceM)
	}
	if len(res.ElapsedGeometry) != 1 {
		t.Errorf("expected elapsed_geometry length 1, got %d", len(res.ElapsedGeometry))
	}
}

func TestPositionAt_End(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.EndAt,
	})
	assertNoError(t, err)

	lastPt := leg.Geometry[len(leg.Geometry)-1]
	assertPointNear(t, lastPt, res.Point, 0.001, "end point")
	if res.ElapsedDistanceM != leg.DistanceM {
		t.Errorf("expected elapsed_distance_m=%d, got %d", leg.DistanceM, res.ElapsedDistanceM)
	}
	if len(res.ElapsedGeometry) != len(leg.Geometry) {
		t.Errorf("expected elapsed_geometry length %d, got %d", len(leg.Geometry), len(res.ElapsedGeometry))
	}
}

func TestPositionAt_Middle(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()

	// 50% elapsed → midpoint of the leg (Lat ≈ 1.0).
	halfTime := leg.StartAt.Add(leg.EndAt.Sub(leg.StartAt) / 2)
	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  halfTime,
	})
	assertNoError(t, err)

	assertInDelta(t, 1.0, res.Point.Lat, 0.01, "midpoint lat")
	assertInDelta(t, 0.0, res.Point.Lon, 0.01, "midpoint lon")

	expectedDistM := int64(math.Round(0.5 * float64(leg.DistanceM)))
	if res.ElapsedDistanceM != expectedDistM {
		t.Errorf("expected elapsed_distance_m=%d, got %d", expectedDistM, res.ElapsedDistanceM)
	}
	if res.ElapsedDurationSec != 300 {
		t.Errorf("expected elapsed_duration_sec=300, got %d", res.ElapsedDurationSec)
	}

	// Elapsed geometry: start + midpoint + interpolated point.
	if len(res.ElapsedGeometry) < 2 {
		t.Errorf("expected elapsed_geometry length >= 2, got %d", len(res.ElapsedGeometry))
	}
}

func TestPositionAt_BeyondEnd(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.EndAt.Add(time.Hour),
	})
	assertNoError(t, err)

	lastPt := leg.Geometry[len(leg.Geometry)-1]
	assertPointNear(t, lastPt, res.Point, 0.001, "clamped to end")
	if res.ElapsedDistanceM != leg.DistanceM {
		t.Errorf("expected full distance %d, got %d", leg.DistanceM, res.ElapsedDistanceM)
	}
	// elapsed_duration_sec clamped to total duration.
	totalDur := int64(leg.EndAt.Sub(leg.StartAt).Seconds())
	if res.ElapsedDurationSec != totalDur {
		t.Errorf("expected clamped duration %d, got %d", totalDur, res.ElapsedDurationSec)
	}
}

func TestPositionAt_BeforeStart(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt.Add(-time.Minute),
	})
	assertNoError(t, err)

	assertPointNear(t, leg.Geometry[0], res.Point, 0.001, "clamped to start")
	if res.ElapsedDurationSec != 0 || res.ElapsedDistanceM != 0 {
		t.Errorf("expected zero progress, got dur=%d dist=%d", res.ElapsedDurationSec, res.ElapsedDistanceM)
	}
}

func TestPositionAt_ZeroDuration(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()
	// Zero duration: StartAt == EndAt → instantly completed.
	leg.EndAt = leg.StartAt

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt.Add(time.Second),
	})
	assertNoError(t, err)

	lastPt := leg.Geometry[len(leg.Geometry)-1]
	assertPointNear(t, lastPt, res.Point, 0.001, "zero duration → end")
	if res.ElapsedDistanceM != leg.DistanceM {
		t.Errorf("expected full distance, got %d", res.ElapsedDistanceM)
	}
}

func TestPositionAt_ZeroDuration_BeforeStart(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()
	leg.EndAt = leg.StartAt

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt.Add(-time.Second),
	})
	assertNoError(t, err)

	assertPointNear(t, leg.Geometry[0], res.Point, 0.001, "zero duration, before start → start")
	if res.ElapsedDurationSec != 0 || res.ElapsedDistanceM != 0 {
		t.Errorf("expected zero progress")
	}
}

func TestPositionAt_QuarterAndThreeQuarters(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()
	dur := leg.EndAt.Sub(leg.StartAt)

	// 25% → Lat ≈ 0.5
	res25, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt.Add(dur / 4),
	})
	assertNoError(t, err)
	assertInDelta(t, 0.5, res25.Point.Lat, 0.02, "25% lat")

	// 75% → Lat ≈ 1.5
	res75, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt.Add(3 * dur / 4),
	})
	assertNoError(t, err)
	assertInDelta(t, 1.5, res75.Point.Lat, 0.02, "75% lat")

	// Monotonicity.
	if res25.ElapsedDistanceM >= res75.ElapsedDistanceM {
		t.Errorf("25%% distance (%d) should be less than 75%% (%d)", res25.ElapsedDistanceM, res75.ElapsedDistanceM)
	}

	// Sum preservation: elapsed + remaining ≈ total.
	remaining25 := leg.DistanceM - res25.ElapsedDistanceM
	remaining75 := leg.DistanceM - res75.ElapsedDistanceM
	if remaining25 < 0 || remaining75 < 0 {
		t.Errorf("remaining distance should be non-negative")
	}
}

func TestPositionAt_InvalidPolyline(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()
	leg.Geometry = []contracts.Point{{Lat: 0, Lon: 0}} // only 1 point

	_, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt,
	})
	assertContractError(t, err, "INVALID_INPUT")
}

func TestPositionAt_Monotonicity(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()
	dur := leg.EndAt.Sub(leg.StartAt)

	prevDist := int64(0)
	for pct := 0; pct <= 100; pct += 5 {
		at := leg.StartAt.Add(time.Duration(float64(dur) * float64(pct) / 100))
		res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{Leg: leg, At: at})
		assertNoError(t, err)

		if res.ElapsedDistanceM < prevDist {
			t.Errorf("distance decreased at %d%%: %d < %d", pct, res.ElapsedDistanceM, prevDist)
		}
		prevDist = res.ElapsedDistanceM
	}
}

func TestPositionAt_ElapsedGeometryContainsStart(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	leg := newSimpleLeg()
	dur := leg.EndAt.Sub(leg.StartAt)

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  leg.StartAt.Add(dur / 2),
	})
	assertNoError(t, err)

	if len(res.ElapsedGeometry) == 0 {
		t.Fatal("elapsed_geometry should not be empty")
	}
	// First point should be the start.
	assertPointNear(t, leg.Geometry[0], res.ElapsedGeometry[0], 0.001, "elapsed_geometry[0]")
	// Last point should be the interpolated point.
	assertPointNear(t, res.Point, res.ElapsedGeometry[len(res.ElapsedGeometry)-1], 0.001, "elapsed_geometry[-1]")
}

func TestPositionAt_ExampleFromSpec(t *testing.T) {
	// Reproduce the exact example from backend_flow.json.
	svc := NewGeoService(&MockRouteProvider{})

	leg := contracts.Leg{
		ID:             "leg-1",
		FromLocationID: "office-1",
		ToLocationID:   "loc-1",
		StartAt:        mustTime("2026-09-17T06:00:00Z"),
		EndAt:          mustTime("2026-09-17T06:15:00Z"),
		DistanceM:      1200,
		GeoContextID:   "geo-1",
		Geometry: []contracts.Point{
			{Lat: 55.75, Lon: 37.61},
			{Lat: 55.76, Lon: 37.61},
		},
	}

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{
		Leg: leg,
		At:  mustTime("2026-09-17T06:07:30Z"),
	})
	assertNoError(t, err)

	// Expected from the example.
	assertInDelta(t, 55.755, res.Point.Lat, 0.001, "example lat")
	assertInDelta(t, 37.61, res.Point.Lon, 0.001, "example lon")
	if res.ElapsedDurationSec != 450 {
		t.Errorf("expected elapsed_duration_sec=450, got %d", res.ElapsedDurationSec)
	}
	if res.ElapsedDistanceM != 600 {
		t.Errorf("expected elapsed_distance_m=600, got %d", res.ElapsedDistanceM)
	}
	if len(res.ElapsedGeometry) != 2 {
		t.Errorf("expected elapsed_geometry length 2, got %d", len(res.ElapsedGeometry))
	}
	if len(res.ElapsedGeometry) == 2 {
		assertPointNear(t, contracts.Point{Lat: 55.75, Lon: 37.61}, res.ElapsedGeometry[0], 0.001, "elapsed_geom[0]")
		assertPointNear(t, contracts.Point{Lat: 55.755, Lon: 37.61}, res.ElapsedGeometry[1], 0.001, "elapsed_geom[1]")
	}
}

func TestPositionAt_ZeroDistanceAndDuration(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	p := contracts.Point{Lat: 42, Lon: 42}

	leg := contracts.Leg{
		ID:       "zero-leg",
		StartAt:  mustTime("2026-09-17T06:00:00Z"),
		EndAt:    mustTime("2026-09-17T06:00:00Z"),
		DistanceM: 0,
		Geometry: []contracts.Point{p, p},
	}

	res, err := svc.PositionAt(context.Background(), contracts.PositionRequest{Leg: leg, At: leg.StartAt})
	assertNoError(t, err)

	if math.IsNaN(res.Point.Lat) || math.IsNaN(res.Point.Lon) {
		t.Fatal("should not produce NaN")
	}
}

// ============================================================================
// BuildMatrix Tests
// ============================================================================

func testLocations() []contracts.Location {
	return []contracts.Location{
		{ID: "office-1", Address: "Москва, офис", Point: contracts.Point{Lat: 55.75, Lon: 37.61}},
		{ID: "loc-1", Address: "Москва, ул. Тверская, 1", Point: contracts.Point{Lat: 55.76, Lon: 37.61}},
		{ID: "loc-2", Address: "Санкт-Петербург", Point: contracts.Point{Lat: 59.93, Lon: 30.31}},
	}
}

func TestBuildMatrix_Dimensions(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()
	locs := testLocations()

	matrix, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: locs,
		Profiles:  []contracts.Transport{contracts.TransportCar},
	})
	assertNoError(t, err)

	carMatrix, ok := matrix.Profiles[contracts.TransportCar]
	if !ok {
		t.Fatal("car profile missing")
	}
	if len(carMatrix) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(carMatrix))
	}
	for i, row := range carMatrix {
		if len(row) != 3 {
			t.Fatalf("row %d: expected 3 columns, got %d", i, len(row))
		}
	}
}

func TestBuildMatrix_DiagonalZeros(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()
	locs := testLocations()[:2]

	matrix, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: locs,
		Profiles:  []contracts.Transport{contracts.TransportCar},
	})
	assertNoError(t, err)

	carMatrix := matrix.Profiles[contracts.TransportCar]
	for i := 0; i < len(locs); i++ {
		cell := carMatrix[i][i]
		if !cell.Reachable {
			t.Errorf("diagonal [%d][%d] should be reachable", i, i)
		}
		if cell.DurationSec == nil || *cell.DurationSec != 0 {
			t.Errorf("diagonal [%d][%d] duration should be 0", i, i)
		}
		if cell.DistanceM == nil || *cell.DistanceM != 0 {
			t.Errorf("diagonal [%d][%d] distance should be 0", i, i)
		}
	}

	// Off-diagonal should be non-zero.
	offDiag := carMatrix[0][1]
	if !offDiag.Reachable || offDiag.DurationSec == nil || *offDiag.DurationSec <= 0 {
		t.Error("off-diagonal cell should be reachable with positive duration")
	}
}

func TestBuildMatrix_MultipleProfiles(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()
	locs := testLocations()[:2]

	matrix, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: locs,
		Profiles:  []contracts.Transport{contracts.TransportCar, contracts.TransportWalk},
	})
	assertNoError(t, err)

	if _, ok := matrix.Profiles[contracts.TransportCar]; !ok {
		t.Error("car profile missing")
	}
	if _, ok := matrix.Profiles[contracts.TransportWalk]; !ok {
		t.Error("walk profile missing")
	}

	// Walk should be slower than car for the same pair.
	carDur := *matrix.Profiles[contracts.TransportCar][0][1].DurationSec
	walkDur := *matrix.Profiles[contracts.TransportWalk][0][1].DurationSec
	if walkDur <= carDur {
		t.Errorf("walk (%d sec) should be slower than car (%d sec)", walkDur, carDur)
	}
}

func TestBuildMatrix_HasIDAndContext(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()

	matrix, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: testLocations()[:2],
		Profiles:  []contracts.Transport{contracts.TransportCar},
	})
	assertNoError(t, err)

	if matrix.ID == "" {
		t.Error("matrix.ID should not be empty")
	}
	if matrix.GeoContextID == "" {
		t.Error("matrix.GeoContextID should not be empty")
	}
	if len(matrix.LocationIDs) != 2 {
		t.Errorf("expected 2 location IDs, got %d", len(matrix.LocationIDs))
	}
}

func TestBuildMatrix_ProviderError(t *testing.T) {
	svc := NewGeoService(NewFailingRouteProvider(errors.New("network timeout")))
	ctx := context.Background()

	_, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: testLocations()[:2],
		Profiles:  []contracts.Transport{contracts.TransportCar},
	})
	assertContractError(t, err, "GEO_UNAVAILABLE")
}

func TestBuildMatrix_Directional(t *testing.T) {
	// A→B may differ from B→A.
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()
	locs := testLocations()[:2]

	matrix, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: locs,
		Profiles:  []contracts.Transport{contracts.TransportCar},
	})
	assertNoError(t, err)

	carMatrix := matrix.Profiles[contracts.TransportCar]
	// For the mock, A→B and B→A distances are the same since it's haversine,
	// but this test verifies both cells are independently populated.
	if !carMatrix[0][1].Reachable || !carMatrix[1][0].Reachable {
		t.Error("both directions should be reachable")
	}
}

func TestBuildMatrix_EmptyInput(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()

	_, err := svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: []contracts.Location{},
		Profiles:  []contracts.Transport{contracts.TransportCar},
	})
	assertContractError(t, err, "INVALID_INPUT")

	_, err = svc.BuildMatrix(ctx, contracts.MatrixRequest{
		Locations: testLocations()[:2],
		Profiles:  []contracts.Transport{},
	})
	assertContractError(t, err, "INVALID_INPUT")
}

// ============================================================================
// BuildRoutes Tests
// ============================================================================

func TestBuildRoutes_Success(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()
	locs := testLocations()[:2]

	res, err := svc.BuildRoutes(ctx, contracts.RoutesRequest{
		GeoContextID: "geo-1",
		Locations:    locs,
		Legs: []contracts.RoutesRequestLeg{
			{LegID: "leg-1", FromLocationID: "office-1", ToLocationID: "loc-1", Profile: contracts.TransportCar},
		},
	})
	assertNoError(t, err)

	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].LegID != "leg-1" {
		t.Errorf("expected leg_id=leg-1, got %s", res.Items[0].LegID)
	}
	if len(res.Items[0].Geometry) < 2 {
		t.Errorf("expected geometry with >= 2 points, got %d", len(res.Items[0].Geometry))
	}
}

func TestBuildRoutes_PartialFailure(t *testing.T) {
	// Second route call fails → entire request fails.
	svc := NewGeoService(NewSelectiveRouteProvider(1))
	ctx := context.Background()
	locs := testLocations()

	_, err := svc.BuildRoutes(ctx, contracts.RoutesRequest{
		GeoContextID: "geo-1",
		Locations:    locs,
		Legs: []contracts.RoutesRequestLeg{
			{LegID: "leg-1", FromLocationID: "office-1", ToLocationID: "loc-1", Profile: contracts.TransportCar},
			{LegID: "leg-2", FromLocationID: "loc-1", ToLocationID: "loc-2", Profile: contracts.TransportCar},
		},
	})
	assertContractError(t, err, "GEO_UNAVAILABLE")
}

func TestBuildRoutes_UnreachableRoute(t *testing.T) {
	svc := NewGeoService(NewUnreachableRouteProvider())
	ctx := context.Background()
	locs := testLocations()[:2]

	_, err := svc.BuildRoutes(ctx, contracts.RoutesRequest{
		GeoContextID: "geo-1",
		Locations:    locs,
		Legs: []contracts.RoutesRequestLeg{
			{LegID: "leg-1", FromLocationID: "office-1", ToLocationID: "loc-1", Profile: contracts.TransportCar},
		},
	})
	assertContractError(t, err, "GEO_UNAVAILABLE")
}

func TestBuildRoutes_UnknownLocationID(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()
	locs := testLocations()[:2]

	_, err := svc.BuildRoutes(ctx, contracts.RoutesRequest{
		GeoContextID: "geo-1",
		Locations:    locs,
		Legs: []contracts.RoutesRequestLeg{
			{LegID: "leg-1", FromLocationID: "nonexistent", ToLocationID: "loc-1", Profile: contracts.TransportCar},
		},
	})
	assertContractError(t, err, "INVALID_INPUT")
}

func TestBuildRoutes_EmptyLegs(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()

	_, err := svc.BuildRoutes(ctx, contracts.RoutesRequest{
		GeoContextID: "geo-1",
		Locations:    testLocations(),
		Legs:         []contracts.RoutesRequestLeg{},
	})
	assertContractError(t, err, "INVALID_INPUT")
}

func TestBuildRoutes_ErrorContainsLegID(t *testing.T) {
	svc := NewGeoService(NewFailingRouteProvider(errors.New("broken")))
	ctx := context.Background()
	locs := testLocations()[:2]

	_, err := svc.BuildRoutes(ctx, contracts.RoutesRequest{
		GeoContextID: "geo-1",
		Locations:    locs,
		Legs: []contracts.RoutesRequestLeg{
			{LegID: "leg-42", FromLocationID: "office-1", ToLocationID: "loc-1", Profile: contracts.TransportCar},
		},
	})
	var ce *contracts.ContractError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ContractError, got %T", err)
	}
	if legID, ok := ce.Details["leg_id"]; !ok || legID != "leg-42" {
		t.Errorf("error details should contain leg_id=leg-42, got %v", ce.Details)
	}
}

// ============================================================================
// Geocode Tests
// ============================================================================

func TestGeocode_Success(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()

	res, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID: "region-1",
		Locations: []contracts.LocationInput{
			{ID: "loc-1", Address: "Москва, ул. Тверская, 1"},
		},
	})
	assertNoError(t, err)

	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.LocationID != "loc-1" {
		t.Errorf("expected location_id=loc-1, got %s", item.LocationID)
	}
	if item.Location == nil {
		t.Fatal("expected resolved location, got nil")
	}
	if item.Issue != nil {
		t.Errorf("expected no issue, got %+v", item.Issue)
	}
}

func TestGeocode_AlreadyHasPoint(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()

	pt := contracts.Point{Lat: 55.76, Lon: 37.61}
	res, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID: "region-1",
		Locations: []contracts.LocationInput{
			{ID: "loc-1", Address: "Москва", Point: &pt},
		},
	})
	assertNoError(t, err)

	item := res.Items[0]
	if item.Location == nil {
		t.Fatal("expected location")
	}
	if item.Location.Point != pt {
		t.Errorf("expected original point %v, got %v", pt, item.Location.Point)
	}
}

func TestGeocode_Ambiguous(t *testing.T) {
	svc := NewGeoService(NewAmbiguousGeocodeProvider())
	ctx := context.Background()

	res, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID: "region-1",
		Locations: []contracts.LocationInput{
			{ID: "loc-1", Address: "Springfield"},
		},
	})
	assertNoError(t, err)

	item := res.Items[0]
	if item.Location != nil {
		t.Error("should not silently pick a candidate")
	}
	if item.Issue == nil {
		t.Fatal("expected issue for ambiguous address")
	}
	if item.Issue.Code != "INVALID_INPUT" {
		t.Errorf("expected code INVALID_INPUT, got %s", item.Issue.Code)
	}
}

func TestGeocode_Unavailable(t *testing.T) {
	svc := NewGeoService(NewEmptyGeocodeProvider())
	ctx := context.Background()

	res, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID: "region-1",
		Locations: []contracts.LocationInput{
			{ID: "loc-1", Address: "Несуществующий адрес"},
		},
	})
	assertNoError(t, err)

	item := res.Items[0]
	if item.Issue == nil {
		t.Fatal("expected issue for unavailable address")
	}
	if item.Issue.Code != "GEO_UNAVAILABLE" {
		t.Errorf("expected code GEO_UNAVAILABLE, got %s", item.Issue.Code)
	}
}

func TestGeocode_ProviderError(t *testing.T) {
	svc := NewGeoService(NewFailingGeocodeProvider(errors.New("DNS failure")))
	ctx := context.Background()

	_, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID: "region-1",
		Locations: []contracts.LocationInput{
			{ID: "loc-1", Address: "Any"},
		},
	})
	assertContractError(t, err, "GEO_UNAVAILABLE")
}

func TestGeocode_MixedResults(t *testing.T) {
	callCount := 0
	provider := &MockRouteProvider{
		GeocodeFunc: func(_ context.Context, address string) ([]contracts.Location, error) {
			callCount++
			if callCount == 1 {
				return []contracts.Location{
					{ID: "r1", Address: address, Point: contracts.Point{Lat: 55.75, Lon: 37.61}},
				}, nil
			}
			return []contracts.Location{}, nil // 0 candidates
		},
	}
	svc := NewGeoService(provider)
	ctx := context.Background()

	res, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID: "region-1",
		Locations: []contracts.LocationInput{
			{ID: "loc-ok", Address: "Good address"},
			{ID: "loc-bad", Address: "Bad address"},
		},
	})
	assertNoError(t, err)

	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(res.Items))
	}
	if res.Items[0].Location == nil {
		t.Error("first item should have resolved location")
	}
	if res.Items[1].Issue == nil || res.Items[1].Issue.Code != "GEO_UNAVAILABLE" {
		t.Error("second item should have GEO_UNAVAILABLE issue")
	}
}

func TestGeocode_EmptyLocations(t *testing.T) {
	svc := NewGeoService(&MockRouteProvider{})
	ctx := context.Background()

	_, err := svc.Geocode(ctx, contracts.GeocodeRequest{
		RegionID:  "region-1",
		Locations: []contracts.LocationInput{},
	})
	assertContractError(t, err, "INVALID_INPUT")
}

// ============================================================================
// Haversine distance sanity check
// ============================================================================

func TestHaversineDistance(t *testing.T) {
	// 1° latitude along meridian ≈ 111,195 m.
	d := haversineDistance(contracts.Point{Lat: 0, Lon: 0}, contracts.Point{Lat: 1, Lon: 0})
	assertInDelta(t, 111195, d, 200, "1° latitude")

	// Same point → 0.
	d0 := haversineDistance(contracts.Point{Lat: 55, Lon: 37}, contracts.Point{Lat: 55, Lon: 37})
	if d0 != 0 {
		t.Errorf("same point distance should be 0, got %f", d0)
	}

	// Symmetry.
	dAB := haversineDistance(contracts.Point{Lat: 0, Lon: 0}, contracts.Point{Lat: 1, Lon: 1})
	dBA := haversineDistance(contracts.Point{Lat: 1, Lon: 1}, contracts.Point{Lat: 0, Lon: 0})
	assertInDelta(t, dAB, dBA, 0.001, "symmetry")
}
