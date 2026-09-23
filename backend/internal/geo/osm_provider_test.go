package geo

import (
	"context"
	"encoding/json"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOSMGeocodeAndPersistentCache(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[{"geometry":{"type":"Point","coordinates":[37.6,55.7]}}]}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	o := OSMOptions{PhotonURL: srv.URL, CarURL: srv.URL, FootURL: srv.URL, MinInterval: time.Nanosecond, CacheDir: dir, CacheTTL: time.Hour}
	p, _ := NewOSMProvider(o)
	if _, e := p.Geocode(context.Background(), "Moscow"); e != nil {
		t.Fatal(e)
	}
	p, _ = NewOSMProvider(o)
	if _, e := p.Geocode(context.Background(), "Moscow"); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("cache entries=%d", len(entries))
	}
}
func TestOSMRouteProfilesAndRounding(t *testing.T) {
	var got []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"Ok","routes":[{"duration":1.2,"distance":4.6,"geometry":{"type":"LineString","coordinates":[[37,55],[38,56]]}}]}`))
	}))
	defer s.Close()
	p, _ := NewOSMProvider(OSMOptions{CarURL: s.URL + "/car", FootURL: s.URL + "/foot", PhotonURL: s.URL, MinInterval: time.Nanosecond})
	for _, tr := range []contracts.Transport{contracts.TransportCar, contracts.TransportWalk} {
		r, e := p.Route(context.Background(), contracts.Point{Lat: 55, Lon: 37}, contracts.Point{Lat: 56, Lon: 38}, tr)
		if e != nil || r.DurationSec != 2 || r.DistanceM != 5 || len(r.Geometry) != 2 {
			t.Fatalf("route=%+v err=%v", r, e)
		}
	}
	if !strings.Contains(got[0], "/car/") || !strings.Contains(got[1], "/foot/") {
		t.Fatalf("paths=%v", got)
	}
}
func TestOSMMatrixChunksAndMapping(t *testing.T) {
	var requests atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		q, _ := url.ParseQuery(r.URL.RawQuery)

		coords := strings.Split(strings.TrimPrefix(r.URL.Path, "/table/v1/driving/"), ";")
		if len(coords) > 100 {
			t.Errorf("coords=%d", len(coords))
		}
		src := strings.Split(q.Get("sources"), ";")
		dst := strings.Split(q.Get("destinations"), ";")
		d := make([][]*float64, len(src))
		m := make([][]*float64, len(src))
		for i := range d {
			d[i] = make([]*float64, len(dst))
			m[i] = make([]*float64, len(dst))
			for j := range d[i] {
				v := float64(i + j + 1)
				d[i][j] = &v
				m[i][j] = &v
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"code": "Ok", "durations": d, "distances": m})
	}))
	defer s.Close()
	p, _ := NewOSMProvider(OSMOptions{CarURL: s.URL, FootURL: s.URL, PhotonURL: s.URL, MinInterval: time.Nanosecond})
	pts := make([]contracts.Point, 101)
	for i := range pts {
		pts[i] = contracts.Point{Lat: float64(i % 80), Lon: float64(i % 170)}
	}
	out, e := p.RouteMatrix(context.Background(), pts, contracts.TransportCar)
	if e != nil {
		t.Fatal(e)
	}
	if len(out) != 101*101 || requests.Load() != 9 {
		t.Fatalf("len=%d requests=%d", len(out), requests.Load())
	}
	if !out[0].Reachable || out[1].DurationSec != 2 {
		t.Fatalf("mapping=%+v", out[1])
	}
}
func TestOSMContextCancellation(t *testing.T) {
	p, _ := NewOSMProvider(OSMOptions{PhotonURL: "http://127.0.0.1:1", CarURL: "http://127.0.0.1:1", FootURL: "http://127.0.0.1:1", MinInterval: time.Hour})
	ctx, c := context.WithCancel(context.Background())
	c()
	if _, e := p.Geocode(ctx, "x"); e == nil {
		t.Fatal("expected cancellation")
	}
}

func TestParseRussianAddressVariants(t *testing.T) {
	tests := []struct{ name, input, city, street, house string }{
		{"yunyh lenincev", "г. Москва, ул Юных Ленинцев, д 83с 4", "москва", "улица юных ленинцев", "83 с4"},
		{"biryulevskaya", "г. Москва, ул Бирюлёвская, д 1с1", "москва", "улица бирюлевская", "1 с1"},
		{"simferopolsky", "г.Москва проезд Симферопольский, д.7", "москва", "проезд симферопольский", "7"},
		{"volgogradsky", "Город Москва, пр-кт.Волгоградский, д. 128 к 5", "москва", "проспект волгоградский", "128 к5"},
		{"gurevsky", "Город Москва, проезд.Гурьевский, д. 23 к 1", "москва", "проезд гурьевский", "23 к1"},
		{"institutskaya", "Город Москва, ул.3-я Институтская, д. 5 к 2", "москва", "улица 3-я институтская", "5 к2"},
		{"domodedovo", "Домодедово, проезд.Советский 1-й, д. 1А", "домодедово", "проезд советский 1-й", "1а"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRussianAddress(tt.input)
			if !got.ok || got.city != tt.city || got.street != tt.street || got.house != tt.house {
				t.Fatalf("parse(%q) = %+v", tt.input, got)
			}
		})
	}
}

func TestOSMNormalizedGeocodeMatchesOnlyCompleteAddress(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[
{"geometry":{"type":"Point","coordinates":[37.774,55.702]},"properties":{"city":"Москва","street":"улица Юных Ленинцев","housenumber":"83 к4"}},
{"geometry":{"type":"Point","coordinates":[37.775,55.703]},"properties":{"city":"Москва","street":"улица Юных Ленинцев","housenumber":"83 с4"}},
{"geometry":{"type":"Point","coordinates":[37.776,55.704]},"properties":{"city":"Оренбург","street":"улица Юных Ленинцев","housenumber":"83 с4"}}]}`))
	}))
	defer srv.Close()
	p, err := NewOSMProvider(OSMOptions{PhotonURL: srv.URL, CarURL: srv.URL, FootURL: srv.URL, MinInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	locations, err := p.Geocode(context.Background(), "г. Москва, ул Юных Ленинцев, д 83с 4")
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("q") != "москва улица юных ленинцев 83 с4" || gotQuery.Get("countrycode") != "RU" {
		t.Fatalf("normalized query = %v", gotQuery)
	}
	if len(locations) != 1 || locations[0].Point.Lat != 55.703 || locations[0].Point.Lon != 37.775 || locations[0].Address != "г. Москва, ул Юных Ленинцев, д 83с 4" {
		t.Fatalf("locations = %+v", locations)
	}
}

func TestAddressMatchesRejectsWrongCityStreetAndBuilding(t *testing.T) {
	wanted := parseRussianAddress("г. Москва, ул Юных Ленинцев, д 83с 4")
	for _, tc := range []struct{ name, city, street, house string }{
		{"wrong city", "Оренбург", "улица Юных Ленинцев", "83 с4"}, {"wrong street", "Москва", "улица Ленина", "83 с4"}, {"wrong building", "Москва", "улица Юных Ленинцев", "83 к4"}, {"missing house", "Москва", "улица Юных Ленинцев", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if addressMatches("", wanted, tc.city, tc.street, tc.house) {
				t.Fatal("unexpected address match")
			}
		})
	}
}
