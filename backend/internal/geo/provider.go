package geo

import (
	"context"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// RouteResult describes a computed route between two points.
// Reachable=false means the route is not possible (e.g., no road connection);
// this is a valid result, not an error. An error from RouteProvider indicates
// a provider-level failure (network, etc.).
type RouteResult struct {
	Reachable   bool
	DurationSec int64
	DistanceM   int64
	Geometry    []contracts.Point
}

// RouteProvider abstracts the external routing/geocoding API.
// GeoService depends on this interface for testability (DI).
// The contract does not prescribe the provider's shape — Go-3 owns it.
type RouteProvider interface {
	// Geocode resolves an address to candidate locations.
	// Returns an empty slice when nothing is found (not an error).
	Geocode(ctx context.Context, address string) ([]contracts.Location, error)

	// Route computes a route between two points for the given transport profile.
	// Returns RouteResult with Reachable=false if no route exists.
	// Returns error only on provider-level failure.
	Route(ctx context.Context, from, to contracts.Point, transport contracts.Transport) (RouteResult, error)
}

// MatrixProvider optionally computes a complete matrix in one provider call.
type MatrixProvider interface {
	RouteMatrix(ctx context.Context, points []contracts.Point, transport contracts.Transport) ([]RouteResult, error)
}

type AddressSearchResult struct {
	Exact       []contracts.AddressCandidate
	Suggestions []contracts.AddressCandidate
}

type AddressSearchProvider interface {
	SearchAddress(context.Context, string) (AddressSearchResult, error)
}
