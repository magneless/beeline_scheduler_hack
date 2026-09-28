package geo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

const maxGeoResponse = 16 << 20
const maxCacheEntries = 512
const maxCacheBytes = 64 << 20

type OSMOptions struct {
	PhotonURL, CarURL, FootURL, UserAgent string
	Client                                *http.Client
	MinInterval                           time.Duration
	CacheDir                              string
	CacheTTL                              time.Duration
}

type OSMProvider struct {
	o       OSMOptions
	client  *http.Client
	gatesMu sync.Mutex
	gates   map[string]*requestGate
	cacheMu sync.Mutex
}

type requestGate struct {
	lock     chan struct{}
	next     time.Time
	interval time.Duration
}

func NewOSMProvider(o OSMOptions) (*OSMProvider, error) {
	if o.PhotonURL == "" {
		o.PhotonURL = "https://photon.komoot.io"
	}
	if o.CarURL == "" {
		o.CarURL = "https://routing.openstreetmap.de/routed-car"
	}
	if o.FootURL == "" {
		o.FootURL = "https://routing.openstreetmap.de/routed-foot"
	}
	if o.UserAgent == "" {
		o.UserAgent = "beeline-scheduler/1.0 (https://github.com/magneless/beeline_scheduler_hack)"
	}
	if o.MinInterval < 0 || o.CacheTTL < 0 {
		return nil, fmt.Errorf("negative OSM interval or cache TTL")
	}
	if o.CacheTTL == 0 {
		o.CacheTTL = 24 * time.Hour
	}
	for _, endpoint := range []string{o.PhotonURL, o.CarURL, o.FootURL} {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("invalid OSM endpoint %q", endpoint)
		}
	}
	if o.Client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSHandshakeTimeout = 15 * time.Second
		transport.IdleConnTimeout = 90 * time.Second
		transport.MaxIdleConnsPerHost = 4
		transport.MaxConnsPerHost = 4
		o.Client = &http.Client{Timeout: 20 * time.Second, Transport: transport}
	}
	if o.CacheDir != "" {
		if err := os.MkdirAll(o.CacheDir, 0750); err != nil {
			return nil, fmt.Errorf("create geo cache: %w", err)
		}
	}
	return &OSMProvider{o: o, client: o.Client, gates: make(map[string]*requestGate)}, nil
}

// Each upstream has its own queue. A slow route request must not block address
// search. Public limits still apply to every request, including retries.
func (p *OSMProvider) requestGate(endpoint string) *requestGate {
	u, _ := url.Parse(endpoint)
	p.gatesMu.Lock()
	defer p.gatesMu.Unlock()
	key := u.Host
	if gate := p.gates[key]; gate != nil {
		return gate
	}
	interval := p.o.MinInterval
	if u.Hostname() == "photon.komoot.io" || u.Hostname() == "routing.openstreetmap.de" {
		interval = max(interval, time.Second)
	}
	gate := &requestGate{lock: make(chan struct{}, 1), interval: interval}
	p.gates[key] = gate
	return gate
}

func (p *OSMProvider) get(ctx context.Context, endpoint string, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if body, ok := p.loadCache(endpoint); ok {
		if err := json.Unmarshal(body, value); err == nil {
			return nil
		}
	}
	gate := p.requestGate(endpoint)
	select {
	case gate.lock <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-gate.lock }()
	// Another request may have populated the cache while we waited.
	if body, ok := p.loadCache(endpoint); ok {
		if err := json.Unmarshal(body, value); err == nil {
			return nil
		}
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt > 0 {
			if err := p.wait(ctx, time.Duration(500*(1<<(attempt-1)))*time.Millisecond); err != nil {
				return err
			}
		}
		if err := p.wait(ctx, time.Until(gate.next)); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", p.o.UserAgent)
		req.Header.Set("Accept", "application/json")
		response, err := p.client.Do(req)
		// Count the upstream interval from completion of the previous attempt.
		// This also keeps requests apart when connection setup and handler
		// scheduling take different amounts of time.
		gate.next = time.Now().Add(gate.interval)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			if !retryableNetError(ctx, err) {
				return err
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxGeoResponse+1))
		response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if !retryableNetError(ctx, readErr) {
				return readErr
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			lastErr = &osmHTTPError{status: response.StatusCode, text: response.Status}
			if !retryableStatus(response.StatusCode) || attempt == 2 {
				return lastErr
			}
			if d, valid := parseRetryAfter(response.Header.Get("Retry-After")); valid {
				if d > 30*time.Second {
					return lastErr
				}
				if err := p.wait(ctx, d); err != nil {
					return err
				}
			}
			continue
		}
		if len(body) > maxGeoResponse {
			return fmt.Errorf("OSM response exceeds size limit")
		}
		if err = json.Unmarshal(body, value); err != nil {
			return fmt.Errorf("invalid OSM JSON: %w", err)
		}
		var header struct {
			Code string `json:"code"`
			Type string `json:"type"`
		}
		if json.Unmarshal(body, &header) == nil && (header.Code == "Ok" || header.Code == "NoRoute" || header.Type == "FeatureCollection") {
			p.saveCache(endpoint, body)
		}
		return nil
	}
	return lastErr
}

func (p *OSMProvider) wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func retryableStatus(status int) bool {
	return status == 408 || status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}
func retryableNetError(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	return (errors.As(err, &ne) && (ne.Timeout() || ne.Temporary())) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(strings.ToLower(err.Error()), "connection reset")
}
func parseRetryAfter(value string) (time.Duration, bool) {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if n < 0 {
			return 0, false
		}
		// Values over the wait budget must stop retries, even if converting
		// a very large number of seconds would overflow time.Duration.
		if n > 30 {
			return 31 * time.Second, true
		}
		return time.Duration(n) * time.Second, true
	}
	if when, err := http.ParseTime(value); err == nil {
		return max(0, time.Until(when)), true
	}
	return 0, false
}

type geoCacheEntry struct {
	At   time.Time       `json:"at"`
	Body json.RawMessage `json:"body"`
}

func (p *OSMProvider) cachePath(endpoint string) string {
	hash := sha256.Sum256([]byte(endpoint))
	return filepath.Join(p.o.CacheDir, fmt.Sprintf("%x.json", hash))
}
func (p *OSMProvider) loadCache(endpoint string) ([]byte, bool) {
	if p.o.CacheDir == "" {
		return nil, false
	}
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	path := p.cachePath(endpoint)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxGeoResponse+1024))
	if err != nil {
		return nil, false
	}
	var entry geoCacheEntry
	if json.Unmarshal(b, &entry) != nil || entry.At.After(time.Now()) || time.Since(entry.At) > p.o.CacheTTL || !json.Valid(entry.Body) {
		os.Remove(path)
		return nil, false
	}
	return entry.Body, true
}
func (p *OSMProvider) saveCache(endpoint string, body []byte) {
	if p.o.CacheDir == "" {
		return
	}
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	b, err := json.Marshal(geoCacheEntry{At: time.Now(), Body: body})
	if err != nil {
		return
	}
	f, err := os.CreateTemp(p.o.CacheDir, ".geo-*")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return
	}
	if f.Close() != nil {
		return
	}
	if os.Rename(name, p.cachePath(endpoint)) != nil {
		return
	}
	entries, _ := os.ReadDir(p.o.CacheDir)
	type item struct {
		path     string
		size     int64
		modified time.Time
	}
	files := []item{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(p.o.CacheDir, entry.Name())
		if time.Since(info.ModTime()) > p.o.CacheTTL {
			os.Remove(path)
			continue
		}
		files = append(files, item{path, info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	for len(files) > maxCacheEntries || total > maxCacheBytes {
		old := files[0]
		files = files[1:]
		os.Remove(old.path)
		total -= old.size
	}
}

func validPoint(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsInf(lat, 0) && !math.IsNaN(lon) && !math.IsInf(lon, 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
func (p *OSMProvider) routeBase(profile contracts.Transport) (string, error) {
	switch profile {
	case contracts.TransportCar:
		return strings.TrimRight(p.o.CarURL, "/"), nil
	case contracts.TransportWalk:
		return strings.TrimRight(p.o.FootURL, "/"), nil
	default:
		return "", fmt.Errorf("unsupported transport %q", profile)
	}
}
func numericMetrics(duration, distance *float64) (int64, int64, error) {
	if duration == nil || distance == nil || math.IsNaN(*duration) || math.IsInf(*duration, 0) || math.IsNaN(*distance) || math.IsInf(*distance, 0) || *duration < 0 || *distance < 0 || *duration >= float64(math.MaxInt64) || *distance >= float64(math.MaxInt64) {
		return 0, 0, fmt.Errorf("invalid OSRM duration or distance")
	}
	return int64(math.Ceil(*duration)), int64(math.Round(*distance)), nil
}
func (p *OSMProvider) Route(ctx context.Context, a, b contracts.Point, profile contracts.Transport) (RouteResult, error) {
	base, err := p.routeBase(profile)
	if err != nil {
		return RouteResult{}, err
	}
	if !validPoint(a.Lat, a.Lon) || !validPoint(b.Lat, b.Lon) {
		return RouteResult{}, fmt.Errorf("invalid coordinates")
	}
	endpoint := fmt.Sprintf("%s/route/v1/driving/%.7f,%.7f;%.7f,%.7f?overview=full&geometries=geojson", base, a.Lon, a.Lat, b.Lon, b.Lat)
	var response struct {
		Code   string `json:"code"`
		Routes []struct {
			Duration *float64 `json:"duration"`
			Distance *float64 `json:"distance"`
			Geometry struct {
				Type        string      `json:"type"`
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	if err := p.get(ctx, endpoint, &response); err != nil {
		return RouteResult{}, err
	}
	if response.Code == "NoRoute" {
		return RouteResult{Reachable: false}, nil
	}
	if response.Code != "Ok" || len(response.Routes) != 1 {
		return RouteResult{}, fmt.Errorf("OSRM route failed: %s", response.Code)
	}
	route := response.Routes[0]
	duration, distance, err := numericMetrics(route.Duration, route.Distance)
	if err != nil {
		return RouteResult{}, err
	}
	if route.Geometry.Type != "LineString" || len(route.Geometry.Coordinates) < 2 {
		return RouteResult{}, fmt.Errorf("missing OSRM route geometry")
	}
	geometry := make([]contracts.Point, len(route.Geometry.Coordinates))
	for i, coord := range route.Geometry.Coordinates {
		if len(coord) != 2 || !validPoint(coord[1], coord[0]) {
			return RouteResult{}, fmt.Errorf("invalid route coordinates")
		}
		geometry[i] = contracts.Point{Lat: coord[1], Lon: coord[0]}
	}
	return RouteResult{Reachable: true, DurationSec: duration, DistanceM: distance, Geometry: geometry}, nil
}
func (p *OSMProvider) RouteMatrix(ctx context.Context, points []contracts.Point, profile contracts.Transport) ([]RouteResult, error) {
	base, err := p.routeBase(profile)
	if err != nil {
		return nil, err
	}
	n := len(points)
	if n == 0 {
		return []RouteResult{}, nil
	}
	for _, point := range points {
		if !validPoint(point.Lat, point.Lon) {
			return nil, fmt.Errorf("invalid coordinates")
		}
	}
	out := make([]RouteResult, n*n)
	size := 50
	if n <= 100 {
		size = n
	}
	for rowStart := 0; rowStart < n; rowStart += size {
		rowEnd := min(rowStart+size, n)
		for colStart := 0; colStart < n; colStart += size {
			colEnd := min(colStart+size, n)
			coords := []string{}
			indices := map[int]int{}
			add := func(global int) string {
				local, ok := indices[global]
				if !ok {
					local = len(coords)
					indices[global] = local
					point := points[global]
					coords = append(coords, fmt.Sprintf("%.7f,%.7f", point.Lon, point.Lat))
				}
				return strconv.Itoa(local)
			}
			sources, destinations := []string{}, []string{}
			for i := rowStart; i < rowEnd; i++ {
				sources = append(sources, add(i))
			}
			for j := colStart; j < colEnd; j++ {
				destinations = append(destinations, add(j))
			}
			query := url.Values{"annotations": {"duration,distance"}, "sources": {strings.Join(sources, ";")}, "destinations": {strings.Join(destinations, ";")}}
			endpoint := fmt.Sprintf("%s/table/v1/driving/%s?%s", base, strings.Join(coords, ";"), query.Encode())
			var response struct {
				Code      string       `json:"code"`
				Durations [][]*float64 `json:"durations"`
				Distances [][]*float64 `json:"distances"`
			}
			if err := p.get(ctx, endpoint, &response); err != nil {
				return nil, err
			}
			if response.Code != "Ok" || len(response.Durations) != rowEnd-rowStart || len(response.Distances) != rowEnd-rowStart {
				return nil, fmt.Errorf("invalid OSRM table response: %s", response.Code)
			}
			for i := rowStart; i < rowEnd; i++ {
				row := i - rowStart
				if len(response.Durations[row]) != colEnd-colStart || len(response.Distances[row]) != colEnd-colStart {
					return nil, fmt.Errorf("invalid OSRM table row")
				}
				for j := colStart; j < colEnd; j++ {
					col := j - colStart
					if i == j {
						out[i*n+j] = RouteResult{Reachable: true}
						continue
					}
					duration, distance := response.Durations[row][col], response.Distances[row][col]
					if duration == nil && distance == nil {
						continue
					}
					d, m, err := numericMetrics(duration, distance)
					if err != nil {
						return nil, err
					}
					out[i*n+j] = RouteResult{Reachable: true, DurationSec: d, DistanceM: m}
				}
			}
		}
	}
	return out, nil
}

type osmHTTPError struct {
	status int
	text   string
}

func (e *osmHTTPError) Error() string { return "OSM HTTP " + e.text }
