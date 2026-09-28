package geo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

// Opt-in check against the real, free provider. Results go to test output only.
func TestLiveAddressDatasets(t *testing.T) {
	if os.Getenv("LCT_LIVE_GEOCODE") != "1" {
		t.Skip("set LCT_LIVE_GEOCODE=1 to check real dataset addresses")
	}
	cache := os.Getenv("LCT_GEOCODE_TEST_CACHE")
	if cache == "" {
		cache = t.TempDir()
	}
	provider, err := NewOSMProvider(OSMOptions{CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	importer := data.Importer{Geo: NewGeoService(provider), Root: "../../../datasets/original"}
	for _, id := range []string{"southeast", "east", "southcentral"} {
		t.Run(id, func(t *testing.T) {
			started := time.Now()
			snapshot, _, err := importer.Demo(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %d resolved, %d require confirmation; %s", id, len(snapshot.Orders), len(snapshot.UnlocatedOrders), time.Since(started))
			seen := map[string]bool{}
			for _, order := range snapshot.Orders {
				if seen[order.ID] {
					t.Fatal("duplicate", order.ID)
				}
				seen[order.ID] = true
			}
			for _, item := range snapshot.UnlocatedOrders {
				if seen[item.Order.ID] || item.Message == "" {
					t.Fatal("missing or duplicate address diagnosis", item.Order.ID)
				}
				seen[item.Order.ID] = true
				options := []string{}
				for i, candidate := range item.Candidates {
					if i == 3 {
						break
					}
					options = append(options, candidate.Address)
				}
				t.Logf("CHECK %s; candidates=%v", item.Address, options)
			}
			control := data.Importer{Geo: NewGeoService(&DemoProvider{}), Root: importer.Root}
			parsed, _, parseErr := control.Demo(context.Background(), id)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if len(seen) != len(parsed.Orders) {
				t.Errorf("input work lost: %d total, want %d valid source orders", len(seen), len(parsed.Orders))
			}
		})
	}
}
