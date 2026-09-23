package geo

import (
	"context"
	"github.com/magneless/beeline_scheduler_hack/internal/contracts"
	"hash/fnv"
)

// DemoProvider is an offline deterministic provider for integration demonstrations.
// Coordinates and straight-line travel are synthetic, not road network results.
type DemoProvider struct{ MockRouteProvider }

func (p *DemoProvider) Geocode(ctx context.Context, address string) ([]contracts.Location, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := fnv.New32a()
	h.Write([]byte(address))
	v := h.Sum32()
	return []contracts.Location{{Address: address, Point: contracts.Point{Lat: 55.65 + float64(v%1000)/10000, Lon: 37.5 + float64((v/1000)%2000)/10000}}}, nil
}
