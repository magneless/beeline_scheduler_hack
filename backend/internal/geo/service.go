package geo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync/atomic"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// --- ID generation ---

var (
	geoCtxCounter int64
	matrixCounter int64
)

func nextGeoContextID() string {
	n := atomic.AddInt64(&geoCtxCounter, 1)
	return fmt.Sprintf("geo-%d", n)
}

func nextMatrixID() string {
	n := atomic.AddInt64(&matrixCounter, 1)
	return fmt.Sprintf("matrix-%d", n)
}

// --- Service implementation ---

type geoServiceImpl struct {
	provider RouteProvider
}

// NewGeoService creates a GeoService backed by the given RouteProvider.
func NewGeoService(provider RouteProvider) contracts.GeoService {
	return &geoServiceImpl{provider: provider}
}

// Keep provider diagnostics in server logs and preserve the cause for callers,
// while returning a useful message in API responses and persisted run errors.
func providerError(ctx context.Context, operation string, err error, message string, details map[string]any) *contracts.ContractError {
	slog.WarnContext(ctx, "geo provider request failed", "operation", operation, "error", err, "details", details)
	return &contracts.ContractError{
		Code: "GEO_UNAVAILABLE", Message: message, Details: details, Cause: err,
	}
}

// --------------------------------------------------------------------------
// Geocode
// --------------------------------------------------------------------------

// Geocode processes a batch of LocationInputs. For each:
//   - If point is already provided → success, use it directly.
//   - If point is nil → geocode the address via provider.
//   - 1 candidate → success.
//   - 0 candidates → Issue with code GEO_UNAVAILABLE.
//   - >1 candidates → Issue with code INVALID_INPUT (no silent pick).
func (s *geoServiceImpl) Geocode(ctx context.Context, input contracts.GeocodeRequest) (contracts.GeocodeResult, error) {
	if len(input.Locations) == 0 {
		return contracts.GeocodeResult{}, &contracts.ContractError{
			Code:    "INVALID_INPUT",
			Message: "locations list is empty",
			Details: map[string]any{},
		}
	}

	items := make([]contracts.GeocodeResultItem, len(input.Locations))
	type cachedSearch struct {
		result AddressSearchResult
		err    error
	}
	searched := map[string]cachedSearch{}
	var searchUnavailable error
	for i, loc := range input.Locations {
		item := contracts.GeocodeResultItem{LocationID: loc.ID}

		if loc.Point != nil {
			if !validPoint(loc.Point.Lat, loc.Point.Lon) {
				id := loc.ID
				item.Issue = &contracts.Issue{EntityID: &id, Code: "INVALID_INPUT", Message: "Координаты адреса вне допустимого диапазона"}
				items[i] = item
				continue
			}
			// Already has coordinates — no geocoding needed.
			resolved := &contracts.Location{
				ID:      loc.ID,
				Address: loc.Address,
				Point:   *loc.Point,
			}
			item.Location = resolved
			items[i] = item
			continue
		}

		// Geocode the address.
		var candidates []contracts.Location
		var err error
		if searcher, ok := s.provider.(AddressSearchProvider); ok {
			key := strings.TrimSpace(loc.Address)
			cached, exists := searched[key]
			if !exists {
				if searchUnavailable != nil {
					cached.err = searchUnavailable
				} else {
					cached.result, cached.err = searcher.SearchAddress(ctx, loc.Address)
				}
				searched[key] = cached
			}
			err = cached.err
			item.Candidates = cached.result.Suggestions
			for _, match := range cached.result.Exact {
				candidates = append(candidates, contracts.Location{Address: loc.Address, Point: match.Point})
			}
		} else {
			candidates, err = s.provider.Geocode(ctx, loc.Address)
		}
		if err != nil {
			if ctx.Err() != nil {
				return contracts.GeocodeResult{}, ctx.Err()
			}
			var inputError *geocodeInputError
			if errors.As(err, &inputError) {
				id := loc.ID
				item.Issue = &contracts.Issue{EntityID: &id, Code: "INVALID_INPUT", Message: inputError.Error()}
				items[i] = item
				continue
			}
			if _, ok := s.provider.(AddressSearchProvider); ok {
				// Preserve already imported work and every unresolved input when
				// the upstream fails. Do not repeat a failing network call for
				// every remaining row; the dispatcher can retry address search.
				if searchUnavailable == nil {
					slog.WarnContext(ctx, "address search unavailable", "error", err)
				}
				searchUnavailable = err
				id := loc.ID
				item.Issue = &contracts.Issue{EntityID: &id, Code: "GEO_UNAVAILABLE", Message: "Поиск адресов временно недоступен. Повторите поиск или укажите точку на карте"}
				items[i] = item
				continue
			}
			return contracts.GeocodeResult{}, providerError(ctx, "geocode", err,
				"Сервис определения адресов временно недоступен. Повторите попытку позже.",
				map[string]any{"entity_id": loc.ID})
		}

		entityID := loc.ID
		switch len(candidates) {
		case 0:
			item.Issue = &contracts.Issue{
				EntityID: &entityID,
				Code:     "GEO_UNAVAILABLE",
				Message:  fmt.Sprintf("Не найден точный дом по адресу %q. Уточните адрес или укажите координаты", loc.Address),
			}
		case 1:
			resolved := candidates[0]
			resolved.ID = loc.ID
			resolved.Address = loc.Address
			item.Location = &resolved
		default:
			item.Issue = &contracts.Issue{
				EntityID: &entityID,
				Code:     "INVALID_INPUT",
				Message:  fmt.Sprintf("Адрес %q неоднозначен: найдено %d подходящих точек. Уточните адрес или укажите координаты", loc.Address, len(candidates)),
			}
		}
		items[i] = item
	}

	return contracts.GeocodeResult{Items: items}, nil
}

// --------------------------------------------------------------------------
// BuildMatrix
// --------------------------------------------------------------------------

// BuildMatrix builds an N×N TravelMatrix for each requested transport profile.
//
// Rules from geo.md:
//   - Cell [i][j] = route from location_ids[i] to location_ids[j].
//   - Calculation is directional: A→B ≠ B→A.
//   - Reachable cell has duration and distance; unreachable has both null.
//   - Diagonal is always reachable with zero values.
//   - All requested profiles must be present.
//   - Provider failure = error of the entire call (not unreachability).
func (s *geoServiceImpl) BuildMatrix(ctx context.Context, input contracts.MatrixRequest) (contracts.TravelMatrix, error) {
	n := len(input.Locations)
	if n == 0 {
		return contracts.TravelMatrix{}, &contracts.ContractError{
			Code:    "INVALID_INPUT",
			Message: "locations list is empty",
			Details: map[string]any{},
		}
	}
	if len(input.Profiles) == 0 {
		return contracts.TravelMatrix{}, &contracts.ContractError{
			Code:    "INVALID_INPUT",
			Message: "profiles list is empty",
			Details: map[string]any{},
		}
	}

	// Resolve geo context.
	geoCtxID := ""
	if input.GeoContextID != nil {
		geoCtxID = *input.GeoContextID
	} else {
		geoCtxID = nextGeoContextID()
	}

	locationIDs := make([]string, n)
	for i, loc := range input.Locations {
		locationIDs[i] = loc.ID
	}

	profiles := make(map[contracts.Transport][][]contracts.TravelCell, len(input.Profiles))

	for _, transport := range input.Profiles {
		matrix := make([][]contracts.TravelCell, n)
		for i := range matrix {
			matrix[i] = make([]contracts.TravelCell, n)
		}
		var results []RouteResult
		var err error
		if batch, ok := s.provider.(MatrixProvider); ok {
			results, err = batch.RouteMatrix(ctx, func() []contracts.Point {
				p := make([]contracts.Point, n)
				for i, l := range input.Locations {
					p[i] = l.Point
				}
				return p
			}(), transport)
			if err == nil && len(results) != n*n {
				err = fmt.Errorf("matrix result has %d cells, want %d", len(results), n*n)
			}
		} else {
			results = make([]RouteResult, n*n)
			for i := 0; i < n && err == nil; i++ {
				for j := 0; j < n; j++ {
					if i == j {
						results[i*n+j] = RouteResult{Reachable: true}
						continue
					}
					results[i*n+j], err = s.provider.Route(ctx, input.Locations[i].Point, input.Locations[j].Point, transport)
					if err != nil {
						break
					}
				}
			}
		}
		if err != nil {
			return contracts.TravelMatrix{}, providerError(ctx, "matrix", err,
				"Не удалось получить время в пути: сервис маршрутов временно недоступен. Повторите расчёт позже.",
				map[string]any{"profile": transport})
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				r := results[i*n+j]
				if i == j {
					zero := int64(0)
					matrix[i][j] = contracts.TravelCell{Reachable: true, DurationSec: &zero, DistanceM: &zero}
					continue
				}
				if !r.Reachable {
					matrix[i][j] = contracts.TravelCell{Reachable: false}
					continue
				}
				if r.DurationSec < 0 || r.DistanceM < 0 {
					return contracts.TravelMatrix{}, &contracts.ContractError{Code: "GEO_UNAVAILABLE", Message: "provider returned invalid route values", Details: map[string]any{}}
				}
				d, m := r.DurationSec, r.DistanceM
				matrix[i][j] = contracts.TravelCell{Reachable: true, DurationSec: &d, DistanceM: &m}
			}
		}
		profiles[transport] = matrix
	}

	return contracts.TravelMatrix{
		ID:           nextMatrixID(),
		GeoContextID: geoCtxID,
		LocationIDs:  locationIDs,
		Profiles:     profiles,
	}, nil
}

// --------------------------------------------------------------------------
// BuildRoutes
// --------------------------------------------------------------------------

// BuildRoutes computes route geometry for requested legs.
//
// Rules:
//   - Does not reorder visits or recompute schedules.
//   - Geometry is built in the same geo context as the matrix.
//   - If geometry for any requested leg is unavailable, the call fails
//     with the leg_id; no partial result.
func (s *geoServiceImpl) BuildRoutes(ctx context.Context, input contracts.RoutesRequest) (contracts.RoutesGeometry, error) {
	if len(input.Legs) == 0 {
		return contracts.RoutesGeometry{}, &contracts.ContractError{
			Code:    "INVALID_INPUT",
			Message: "legs list is empty",
			Details: map[string]any{},
		}
	}

	// Build location lookup.
	locMap := make(map[string]contracts.Point, len(input.Locations))
	for _, loc := range input.Locations {
		locMap[loc.ID] = loc.Point
	}

	items := make([]contracts.RoutesGeometryItem, len(input.Legs))
	for i, leg := range input.Legs {
		fromPoint, ok := locMap[leg.FromLocationID]
		if !ok {
			return contracts.RoutesGeometry{}, &contracts.ContractError{
				Code:    "INVALID_INPUT",
				Message: fmt.Sprintf("unknown from_location_id %q in leg %q", leg.FromLocationID, leg.LegID),
				Details: map[string]any{"leg_id": leg.LegID},
			}
		}
		toPoint, ok := locMap[leg.ToLocationID]
		if !ok {
			return contracts.RoutesGeometry{}, &contracts.ContractError{
				Code:    "INVALID_INPUT",
				Message: fmt.Sprintf("unknown to_location_id %q in leg %q", leg.ToLocationID, leg.LegID),
				Details: map[string]any{"leg_id": leg.LegID},
			}
		}

		result, err := s.provider.Route(ctx, fromPoint, toPoint, leg.Profile)
		if err != nil {
			return contracts.RoutesGeometry{}, providerError(ctx, "geometry", err,
				"Не удалось загрузить маршруты: сервис маршрутов временно недоступен. Повторите расчёт позже.",
				map[string]any{"leg_id": leg.LegID})
		}
		if !result.Reachable {
			return contracts.RoutesGeometry{}, &contracts.ContractError{
				Code:    "GEO_UNAVAILABLE",
				Message: fmt.Sprintf("route unreachable for leg %q", leg.LegID),
				Details: map[string]any{"leg_id": leg.LegID},
			}
		}

		items[i] = contracts.RoutesGeometryItem{
			LegID:    leg.LegID,
			Geometry: result.Geometry,
		}
	}

	return contracts.RoutesGeometry{Items: items}, nil
}

// --------------------------------------------------------------------------
// PositionAt
// --------------------------------------------------------------------------

// PositionAt estimates a position along a leg's polyline at a given time,
// using uniform (linear) interpolation.
//
// Algorithm:
//  1. Compute totalDuration = EndAt − StartAt and elapsed = At − StartAt.
//  2. Edge cases:
//     - Zero duration → leg is instantly completed.
//     - elapsed ≤ 0 → return start with zero progress.
//     - elapsed ≥ totalDuration → return end with full progress.
//  3. ratio = elapsed / totalDuration.
//  4. Walk the polyline computing cumulative haversine distances.
//  5. targetDist = ratio × totalPolylineLength. Interpolate within
//     the segment that contains the target distance.
//  6. ElapsedDistanceM = round(ratio × leg.DistanceM). This preserves
//     the integer sum: elapsed + remaining = leg.DistanceM.
//  7. ElapsedGeometry = polyline from start to the interpolated point.
func (s *geoServiceImpl) PositionAt(_ context.Context, input contracts.PositionRequest) (contracts.PositionResult, error) {
	leg := input.Leg
	at := input.At

	if len(leg.Geometry) < 2 {
		return contracts.PositionResult{}, &contracts.ContractError{
			Code:    "INVALID_INPUT",
			Message: "leg geometry must have at least 2 points",
			Details: map[string]any{"leg_id": leg.ID},
		}
	}

	totalDurSec := leg.EndAt.Sub(leg.StartAt).Seconds()
	elapsedSec := at.Sub(leg.StartAt).Seconds()

	startPoint := leg.Geometry[0]
	endPoint := leg.Geometry[len(leg.Geometry)-1]

	// Edge case: zero or negative planned duration → leg is instantly completed.
	if totalDurSec <= 0 {
		if elapsedSec < 0 {
			return contracts.PositionResult{
				Point:              startPoint,
				ElapsedDurationSec: 0,
				ElapsedDistanceM:   0,
				ElapsedGeometry:    []contracts.Point{startPoint},
			}, nil
		}
		return contracts.PositionResult{
			Point:              endPoint,
			ElapsedDurationSec: int64(totalDurSec),
			ElapsedDistanceM:   leg.DistanceM,
			ElapsedGeometry:    copyPoints(leg.Geometry),
		}, nil
	}

	// Before start: return start with zero progress.
	if elapsedSec <= 0 {
		return contracts.PositionResult{
			Point:              startPoint,
			ElapsedDurationSec: 0,
			ElapsedDistanceM:   0,
			ElapsedGeometry:    []contracts.Point{startPoint},
		}, nil
	}

	// After end: return end with full progress.
	// elapsed_duration_sec is always clamped to the planned duration.
	if elapsedSec >= totalDurSec {
		return contracts.PositionResult{
			Point:              endPoint,
			ElapsedDurationSec: int64(totalDurSec),
			ElapsedDistanceM:   leg.DistanceM,
			ElapsedGeometry:    copyPoints(leg.Geometry),
		}, nil
	}

	// --- Core interpolation ---
	ratio := elapsedSec / totalDurSec

	// Compute total polyline length by summing segment haversine distances.
	totalPolyLen := 0.0
	for i := 0; i < len(leg.Geometry)-1; i++ {
		totalPolyLen += haversineDistance(leg.Geometry[i], leg.Geometry[i+1])
	}

	targetDist := ratio * totalPolyLen

	// Walk along polyline segments.
	accumulated := 0.0
	for i := 0; i < len(leg.Geometry)-1; i++ {
		segStart := leg.Geometry[i]
		segEnd := leg.Geometry[i+1]
		segLen := haversineDistance(segStart, segEnd)

		if accumulated+segLen >= targetDist {
			// Target point is within this segment.
			remaining := targetDist - accumulated
			var t float64
			if segLen > 0 {
				t = remaining / segLen
			}

			interpPoint := contracts.Point{
				Lat: segStart.Lat + t*(segEnd.Lat-segStart.Lat),
				Lon: segStart.Lon + t*(segEnd.Lon-segStart.Lon),
			}

			// ElapsedDistanceM = round(ratio × DistanceM).
			// Rounding preserves the sum: elapsed + (DistanceM - elapsed) = DistanceM.
			elapsedDistM := int64(math.Round(ratio * float64(leg.DistanceM)))

			// Build elapsed geometry: all points up to current segment + interpolated point.
			elapsedGeom := make([]contracts.Point, 0, i+2)
			elapsedGeom = append(elapsedGeom, leg.Geometry[:i+1]...)
			elapsedGeom = append(elapsedGeom, interpPoint)

			return contracts.PositionResult{
				Point:              interpPoint,
				ElapsedDurationSec: int64(elapsedSec),
				ElapsedDistanceM:   elapsedDistM,
				ElapsedGeometry:    elapsedGeom,
			}, nil
		}

		accumulated += segLen
	}

	// Floating-point rounding fallback: treat as end.
	return contracts.PositionResult{
		Point:              endPoint,
		ElapsedDurationSec: int64(totalDurSec),
		ElapsedDistanceM:   leg.DistanceM,
		ElapsedGeometry:    copyPoints(leg.Geometry),
	}, nil
}

// --- Helpers ---

// haversineDistance returns the great-circle distance in meters between two points.
func haversineDistance(a, b contracts.Point) float64 {
	const earthRadius = 6_371_000.0 // meters

	dLat := degToRad(b.Lat - a.Lat)
	dLon := degToRad(b.Lon - a.Lon)

	lat1 := degToRad(a.Lat)
	lat2 := degToRad(b.Lat)

	sinDLat := math.Sin(dLat / 2)
	sinDLon := math.Sin(dLon / 2)

	h := sinDLat*sinDLat + math.Cos(lat1)*math.Cos(lat2)*sinDLon*sinDLon
	return 2 * earthRadius * math.Asin(math.Sqrt(h))
}

func degToRad(deg float64) float64 {
	return deg * math.Pi / 180
}

func copyPoints(pts []contracts.Point) []contracts.Point {
	cp := make([]contracts.Point, len(pts))
	copy(cp, pts)
	return cp
}

func int64Ptr(v int64) *int64 {
	return &v
}
