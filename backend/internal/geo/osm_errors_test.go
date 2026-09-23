package geo

import (
	"context"
	"encoding/json"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOSMRouteErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{{"query", `{"code":"InvalidQuery"}`, "failed"}, {"geometry", `{"code":"Ok","routes":[{"duration":1,"distance":2,"geometry":{"type":"LineString","coordinates":[]}}]}`, "geometry"}, {"negative", `{"code":"Ok","routes":[{"duration":-1,"distance":2,"geometry":{"type":"LineString","coordinates":[[1,1],[2,2]]}}]}`, "invalid"}, {"overflow", `{"code":"Ok","routes":[{"duration":1e30,"distance":2,"geometry":{"type":"LineString","coordinates":[[1,1],[2,2]]}}]}`, "invalid"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tc.body))
			}))
			defer s.Close()
			p, _ := NewOSMProvider(OSMOptions{CarURL: s.URL, FootURL: s.URL, PhotonURL: s.URL, MinInterval: time.Nanosecond})
			_, e := p.Route(context.Background(), contracts.Point{Lat: 1, Lon: 1}, contracts.Point{Lat: 2, Lon: 2}, contracts.TransportCar)
			if e == nil || !strings.Contains(strings.ToLower(e.Error()), tc.want) {
				t.Fatalf("error=%v", e)
			}
		})
	}
}
func TestOSMNoRouteAndTableNulls(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/route/") {
			w.Write([]byte(`{"code":"NoRoute"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": "Ok", "durations": [][]*float64{{ptr(0), nil}, {ptr(4), ptr(0)}}, "distances": [][]*float64{{ptr(0), nil}, {ptr(8), ptr(0)}}})
	}))
	defer s.Close()
	p, _ := NewOSMProvider(OSMOptions{CarURL: s.URL, FootURL: s.URL, PhotonURL: s.URL, MinInterval: time.Nanosecond})
	r, e := p.Route(context.Background(), contracts.Point{Lat: 1, Lon: 1}, contracts.Point{Lat: 2, Lon: 2}, contracts.TransportCar)
	if e != nil || r.Reachable {
		t.Fatalf("noroute=%+v %v", r, e)
	}
	m, e := p.RouteMatrix(context.Background(), []contracts.Point{{Lat: 1, Lon: 1}, {Lat: 2, Lon: 2}}, contracts.TransportCar)
	if e != nil || m[1].Reachable || !m[2].Reachable {
		t.Fatalf("matrix=%+v %v", m, e)
	}
}
func ptr(v float64) *float64 { return &v }
func TestOSMWaitCancellationAndSpacing(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer s.Close()
	p, _ := NewOSMProvider(OSMOptions{PhotonURL: s.URL, CarURL: s.URL, FootURL: s.URL, MinInterval: 20 * time.Millisecond})
	ctx, c := context.WithCancel(context.Background())
	defer c()
	if _, e := p.Geocode(ctx, "a"); e != nil {
		t.Fatal(e)
	}
	c()
	if _, e := p.Geocode(ctx, "b"); e == nil {
		t.Fatal("expected cancellation")
	}
	if calls.Load() != 1 {
		t.Fatal(calls.Load())
	}
}
func TestOSMCacheTTL(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer s.Close()
	p, _ := NewOSMProvider(OSMOptions{PhotonURL: s.URL, CarURL: s.URL, FootURL: s.URL, MinInterval: time.Nanosecond, CacheDir: t.TempDir(), CacheTTL: time.Millisecond})
	_, _ = p.Geocode(context.Background(), "a")
	time.Sleep(5 * time.Millisecond)
	_, _ = p.Geocode(context.Background(), "a")
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestPhotonExactAddressExcludesFuzzyHouses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"FeatureCollection","features":[{"geometry":{"type":"Point","coordinates":[37.608,55.761]},"properties":{"city":"Москва","street":"Тверская улица","housenumber":"13"}},{"geometry":{"type":"Point","coordinates":[37.608,55.761]},"properties":{"city":"Москва","street":"Тверская улица","housenumber":"13"}},{"geometry":{"type":"Point","coordinates":[37.600,55.765]},"properties":{"city":"Москва","street":"Тверская улица","housenumber":"21 с13"}},{"geometry":{"type":"Point","coordinates":[37.595,55.772]},"properties":{"city":"Москва","street":"3-я Тверская-Ямская улица","housenumber":"13"}}]}`))
	}))
	defer server.Close()
	provider, err := NewOSMProvider(OSMOptions{PhotonURL: server.URL, MinInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	points, err := provider.Geocode(context.Background(), "Москва, Тверская улица, 13")
	if err != nil || len(points) != 1 || points[0].Point.Lon != 37.608 {
		t.Fatalf("points=%+v error=%v", points, err)
	}
	points, err = provider.Geocode(context.Background(), "Тверская")
	if err != nil || len(points) != 3 {
		t.Fatalf("ambiguous address silently resolved: %+v %v", points, err)
	}
}

func TestOSMConcurrentRequestsRespectInterval(t *testing.T) {
	arrivals := make(chan time.Time, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivals <- time.Now()
		w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer server.Close()
	provider, err := NewOSMProvider(OSMOptions{PhotonURL: server.URL, MinInterval: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	errors := make(chan error, 3)
	for _, address := range []string{"a", "b", "c"} {
		go func(address string) { _, err := provider.Geocode(context.Background(), address); errors <- err }(address)
	}
	for range 3 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	previous := <-arrivals
	for range 2 {
		next := <-arrivals
		if next.Sub(previous) < 25*time.Millisecond {
			t.Fatalf("requests too close: %v", next.Sub(previous))
		}
		previous = next
	}
}
