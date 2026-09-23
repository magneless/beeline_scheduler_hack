package geo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

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
	gate    chan struct{}
	next    time.Time
	cacheMu sync.Mutex
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
	if o.MinInterval == 0 {
		o.MinInterval = time.Second
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
		if (u.Hostname() == "photon.komoot.io" || u.Hostname() == "routing.openstreetmap.de") && o.MinInterval < time.Second {
			o.MinInterval = time.Second
		}
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 20 * time.Second}
	}
	if o.CacheDir != "" {
		if err := os.MkdirAll(o.CacheDir, 0750); err != nil {
			return nil, fmt.Errorf("create geo cache: %w", err)
		}
	}
	return &OSMProvider{o: o, client: o.Client, gate: make(chan struct{}, 1)}, nil
}

// Serialize requests, including their start times, across both routing profiles.
func (p *OSMProvider) get(ctx context.Context, endpoint string, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if body, ok := p.loadCache(endpoint); ok {
		if err := json.Unmarshal(body, value); err == nil {
			return nil
		}
	}
	select {
	case p.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-p.gate }()
	// Another request may have populated the cache while we waited.
	if body, ok := p.loadCache(endpoint); ok {
		if err := json.Unmarshal(body, value); err == nil {
			return nil
		}
	}
	if delay := time.Until(p.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", p.o.UserAgent)
	req.Header.Set("Accept", "application/json")
	p.next = time.Now().Add(p.o.MinInterval)
	response, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("OSM HTTP %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGeoResponse+1))
	if err != nil {
		return err
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
func (p *OSMProvider) Geocode(ctx context.Context, address string) ([]contracts.Location, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("empty address")
	}
	parts := parseRussianAddress(address)
	endpoint := strings.TrimRight(p.o.PhotonURL, "/") + "/api/?q=" + url.QueryEscape(address) + "&limit=5"
	if parts.ok {
		v := url.Values{}
		// Photon /structured currently rejects spaced Russian house numbers
		// such as "128 к5". Use the normalized free-text query and validate
		// every returned address component below.
		v.Set("q", parts.city+" "+parts.street+" "+parts.house)
		v.Set("countrycode", "RU")
		v.Set("limit", "5")
		endpoint = strings.TrimRight(p.o.PhotonURL, "/") + "/api/?" + v.Encode()
	}
	var response struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Street string `json:"street"`
				House  string `json:"housenumber"`
				City   string `json:"city"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := p.get(ctx, endpoint, &response); err != nil {
		return nil, err
	}
	if response.Type != "FeatureCollection" || response.Features == nil {
		return nil, fmt.Errorf("malformed Photon response")
	}
	out := make([]contracts.Location, 0, len(response.Features))
	seen := map[contracts.Point]bool{}
	for _, feature := range response.Features {
		coord := feature.Geometry.Coordinates
		if feature.Geometry.Type != "Point" || len(coord) != 2 || !validPoint(coord[1], coord[0]) {
			return nil, fmt.Errorf("invalid Photon coordinates")
		}
		point := contracts.Point{Lat: coord[1], Lon: coord[0]}
		props := feature.Properties
		if addressMatches(address, parts, props.City, props.Street, props.House) && !seen[point] {
			out = append(out, contracts.Location{Address: address, Point: point})
			seen[point] = true
		}
	}
	return out, nil
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

// Photon returns nearby and fuzzy matches too. Only accept the complete
// requested city, street and house; a street centroid is not a house location.
type russianAddress struct {
	city, street, house string
	ok                  bool
}

var (
	houseMarkerRE  = regexp.MustCompile(`(?i)(?:^|[ ,])(?:дом|д)\.?\s*([^,;]+)$`)
	houseTailRE    = regexp.MustCompile(`(?i)(?:^|[ ,])([0-9]+(?:[/\-][0-9]+)?[а-яa-z]?(?:\s*(?:корпус|корп|к|строение|стр|с)\.?\s*[0-9]+[а-яa-z]?){0,2})$`)
	housePartsRE   = regexp.MustCompile(`^([0-9]+(?:[/\-][0-9]+)?[а-яa-z]?)(?:к([0-9]+[а-яa-z]?))?(?:с([0-9]+[а-яa-z]?))?$`)
	streetMarkerRE = regexp.MustCompile(`(?:^| )(?:улица|ул|пр-кт|проспект|просп|проезд|переулок|пер|шоссе|ш|площадь|пл|набережная|наб|бульвар|бул) `)
)

func addressWords(s string) []string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
}

func normAddressPart(s string) string {
	words := addressWords(s)
	out := make([]string, 0, len(words))
	for _, word := range words {
		switch word {
		case "г", "город":
			continue
		case "ул":
			word = "улица"
		case "пр-кт", "просп":
			word = "проспект"
		case "пер":
			word = "переулок"
		case "ш":
			word = "шоссе"
		case "пл":
			word = "площадь"
		case "наб":
			word = "набережная"
		case "бул":
			word = "бульвар"
		}
		out = append(out, word)
	}
	return strings.Join(out, " ")
}

func normHouse(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	s = strings.NewReplacer("корпус", "к", "корп", "к", "строение", "с", "стр", "с", ".", "").Replace(s)
	s = strings.Join(strings.Fields(s), "")
	parts := housePartsRE.FindStringSubmatch(s)
	if parts == nil {
		return ""
	}
	house := parts[1]
	if parts[2] != "" {
		house += " к" + parts[2]
	}
	if parts[3] != "" {
		house += " с" + parts[3]
	}
	return house
}

func parseRussianAddress(address string) russianAddress {
	raw := strings.TrimSpace(address)
	m := houseMarkerRE.FindStringSubmatchIndex(raw)
	if m == nil {
		m = houseTailRE.FindStringSubmatchIndex(raw)
	}
	if m == nil {
		return russianAddress{}
	}
	house := normHouse(raw[m[2]:m[3]])
	before := strings.Trim(raw[:m[0]], " ,;")
	segments := strings.SplitN(before, ",", 2)
	var city, street string
	if len(segments) == 2 {
		city, street = normAddressPart(segments[0]), normAddressPart(segments[1])
	} else {
		before = normAddressPart(before)
		if marker := streetMarkerRE.FindStringIndex(before); marker != nil {
			city, street = strings.TrimSpace(before[:marker[0]]), strings.TrimSpace(before[marker[0]:])
		}
	}
	if city == "" || street == "" || house == "" {
		return russianAddress{}
	}
	return russianAddress{city: city, street: street, house: house, ok: true}
}

func addressMatches(_ string, wanted russianAddress, city, street, house string) bool {
	if !wanted.ok || normAddressPart(city) != wanted.city || normHouse(house) != wanted.house {
		return false
	}
	// OSM can place a street type or ordinal before or after the name.
	left, right := strings.Fields(normAddressPart(street)), strings.Fields(wanted.street)
	sort.Strings(left)
	sort.Strings(right)
	return strings.Join(left, " ") == strings.Join(right, " ")
}
