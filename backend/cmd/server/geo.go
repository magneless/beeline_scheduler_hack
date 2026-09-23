package main

import (
	"fmt"
	"strconv"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/geo"
)

func configuredGeo() (c.GeoService, []c.Issue, error) {
	switch env("GEO_PROVIDER", "osm") {
	case "demo":
		return geo.NewGeoService(&geo.DemoProvider{}), []c.Issue{{Code: "DEMO_GEO", Message: "Координаты демонстрационные; поездки рассчитаны по прямой"}}, nil
	case "osm":
		interval, err := strconv.Atoi(env("GEO_MIN_INTERVAL_MS", "1000"))
		if err != nil || interval < 1 {
			return nil, nil, fmt.Errorf("GEO_MIN_INTERVAL_MS must be a positive integer")
		}
		provider, err := geo.NewOSMProvider(geo.OSMOptions{
			PhotonURL:   env("GEO_PHOTON_URL", "https://photon.komoot.io"),
			CarURL:      env("GEO_OSRM_CAR_URL", "https://routing.openstreetmap.de/routed-car"),
			FootURL:     env("GEO_OSRM_FOOT_URL", "https://routing.openstreetmap.de/routed-foot"),
			UserAgent:   env("GEO_USER_AGENT", "beeline-scheduler/1.0 (https://github.com/magneless/beeline_scheduler_hack)"),
			MinInterval: time.Duration(interval) * time.Millisecond,
			CacheDir:    env("GEO_CACHE_DIR", ".cache/geo"),
			CacheTTL:    24 * time.Hour,
		})
		if err != nil {
			return nil, nil, err
		}
		return geo.NewGeoService(provider), nil, nil
	default:
		return nil, nil, fmt.Errorf("GEO_PROVIDER must be osm or demo")
	}
}
