package contracts

import (
	"context"
	"time"
)

// GeoService provides geocoding, travel matrix computation, route geometry,
// and position-on-route calculations.
//
// Consumers: Go-2 (Geocode for address normalization), Go-4 (all methods).
// Go-1 receives the matrix indirectly through Go-4.
type GeoService interface {
	Geocode(ctx context.Context, input GeocodeRequest) (GeocodeResult, error)
	BuildMatrix(ctx context.Context, input MatrixRequest) (TravelMatrix, error)
	BuildRoutes(ctx context.Context, input RoutesRequest) (RoutesGeometry, error)
	PositionAt(ctx context.Context, input PositionRequest) (PositionResult, error)
}

// --- Geocode ---

// GeocodeRequest is a batch geocoding request.
type GeocodeRequest struct {
	RegionID  string          `json:"region_id"`
	Locations []LocationInput `json:"locations"`
}

// GeocodeResultItem holds the result for a single input location.
// Exactly one of Location or Issue is non-nil.
type GeocodeResultItem struct {
	LocationID string    `json:"location_id"`
	Location   *Location `json:"location"`
	Issue      *Issue    `json:"issue"`
}

// GeocodeResult is the response for a geocoding request.
type GeocodeResult struct {
	Items []GeocodeResultItem `json:"items"`
}

// --- Matrix ---

// MatrixRequest is a request to build an N×N travel matrix for given locations
// and transport profiles.
type MatrixRequest struct {
	Locations    []Location  `json:"locations"`
	Profiles     []Transport `json:"profiles"`
	GeoContextID *string     `json:"geo_context_id"`
}

// --- Routes ---

// RoutesRequestLeg describes a single leg for route geometry computation.
type RoutesRequestLeg struct {
	LegID          string    `json:"leg_id"`
	FromLocationID string    `json:"from_location_id"`
	ToLocationID   string    `json:"to_location_id"`
	Profile        Transport `json:"profile"`
}

// RoutesRequest is a request to build route geometry for a set of legs.
type RoutesRequest struct {
	GeoContextID string      `json:"geo_context_id"`
	Locations    []Location  `json:"locations"`
	Legs         []RoutesRequestLeg `json:"legs"`
}

// RoutesGeometryItem holds the geometry for a single leg.
type RoutesGeometryItem struct {
	LegID    string  `json:"leg_id"`
	Geometry []Point `json:"geometry"`
}

// RoutesGeometry is the response for a routes request.
type RoutesGeometry struct {
	Items []RoutesGeometryItem `json:"items"`
}

// --- Position ---

// PositionRequest asks for the estimated position along a leg at a given time.
type PositionRequest struct {
	Leg Leg       `json:"leg"`
	At  time.Time `json:"at"`
}

// PositionResult describes the estimated position along a leg.
type PositionResult struct {
	Point              Point   `json:"point"`
	ElapsedDurationSec int64   `json:"elapsed_duration_sec"`
	ElapsedDistanceM   int64   `json:"elapsed_distance_m"`
	ElapsedGeometry    []Point `json:"elapsed_geometry"`
}
