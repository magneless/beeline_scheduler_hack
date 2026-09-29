package geo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestCompoundHouseStructuredSearchChecksFullNumber(t *testing.T) {
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/structured" {
			photonReply(w)
			return
		}
		if r.URL.Query().Get("housenumber") != "128" {
			t.Errorf("missing family search: %s", r.URL)
		}
		photonReply(w, photonFeature(37.60, "Волгоградский проспект", "128 к1"), photonFeature(37.61, "Волгоградский проспект", "128 к5"))
	})
	got, err := p.Geocode(context.Background(), "Город Москва, пр-кт.Волгоградский, д. 128 к 5")
	if err != nil || len(got) != 1 || got[0].Point.Lon != 37.61 {
		t.Fatalf("%v %v", got, err)
	}
}
func TestAddressHierarchyAndApartment(t *testing.T) {
	for _, address := range []string{"Россия, 142007, Московская область, г. Домодедово, пгт.Востряково-1, ул.Жуковского, д.14/18", "обл.Московская область, г.Домодедово, пгт.Востряково-1, ул.Жуковского, д. 14/18, кв. 21"} {
		parts := parseRussianAddress(address)
		if !parts.ok || parts.city != "домодедово" || parts.street != "улица жуковского" || parts.house != "14 к18" {
			t.Fatalf("%s: %+v", address, parts)
		}
	}
}

func TestUnusualAddressMatchesCompleteCandidate(t *testing.T) {
	for _, tc := range []struct{ address, city, street, house string }{
		{"Город Москва, б-р.Самаркандский Квартал 137а, д. к5", "Москва", "квартал Самаркандский Бульвар 137А", "к5"},
		{"г. Великий Новгород Большая Московская ул. д. 5", "Великий Новгород", "Большая Московская улица", "5"},
		{"г. Великий Новгород Большая Московская ул. д. 5 / 1", "Великий Новгород", "Большая Московская улица", "5 корпус 1"},
		{"Москва, улица 8-го Марта, д. 4, кв. 12", "Москва", "улица 8 Марта", "4"},
	} {
		t.Run(tc.address, func(t *testing.T) {
			var f photonFeatureResult
			f.Properties.City, f.Properties.Street, f.Properties.House = tc.city, tc.street, tc.house
			if !candidateMatches(tc.address, parseRussianAddress(tc.address), f) {
				t.Fatal("complete address was rejected")
			}
			f.Properties.House = "99"
			if candidateMatches(tc.address, parseRussianAddress(tc.address), f) {
				t.Fatal("different building was accepted")
			}
		})
	}
}

func TestPhotonExtentOrderDeduplicatesSuggestions(t *testing.T) {
	var building photonFeatureResult
	building.Geometry.Coordinates = []float64{37.61, 55.76}
	building.Properties.Street, building.Properties.House = "улица Петровка", "2"
	building.Properties.OSMKey = "building"
	// Photon uses top-left and bottom-right corners, unlike GeoJSON bbox.
	building.Properties.Extent = []float64{37.609, 55.761, 37.611, 55.759}
	shop := building
	shop.Properties.OSMKey = "shop"
	shop.Geometry.Coordinates = []float64{37.6101, 55.7601}
	if got := dedupeAddressObjects([]photonFeatureResult{shop, building}); len(got) != 1 || got[0].Properties.OSMKey != "building" {
		t.Fatalf("duplicate house candidates: %+v", got)
	}
	other := building
	other.Geometry.Coordinates = []float64{37.6102, 55.7602}
	if got := dedupeAddressObjects([]photonFeatureResult{other, building}); len(got) != 2 {
		t.Fatal("distinct building footprints were silently merged")
	}
}
func TestDuplicateBuildingAndShopAreOneAddress(t *testing.T) {
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		features := []any{
			map[string]any{"geometry": map[string]any{"type": "Point", "coordinates": []float64{37.6101, 55.7601}}, "properties": map[string]any{"city": "Москва", "street": "улица Петровка", "housenumber": "2", "osm_type": "N", "osm_id": 2, "osm_key": "shop"}},
			map[string]any{"geometry": map[string]any{"type": "Point", "coordinates": []float64{37.61, 55.76}}, "properties": map[string]any{"city": "Москва", "street": "улица Петровка", "housenumber": "2", "osm_type": "W", "osm_id": 1, "osm_key": "building", "extent": []float64{37.609, 55.759, 37.611, 55.761}}},
		}
		photonReply(w, features...)
	})
	got, err := p.Geocode(context.Background(), "Москва, Петровка, 2")
	if err != nil || len(got) != 1 || got[0].Point.Lon != 37.61 {
		t.Fatalf("%v %v", got, err)
	}
}
func TestBuildingSuffixMismatchIsSuggestedNotAssigned(t *testing.T) {
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		photonReply(w, photonFeature(37.61, "улица Юных Ленинцев", "83 к4"))
	})
	got, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "one", Address: "Москва, Юных Ленинцев, 83с4"}}})
	if err != nil || got.Items[0].Location != nil || len(got.Items[0].Candidates) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestGeocodeNearbyExactCandidates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		lons   []float64
		houses []string
		issue  string
	}{
		{name: "first in provider order", lons: []float64{37.6109, 37.61}},
		{name: "reversed provider order", lons: []float64{37.61, 37.6109}},
		{name: "just below 100 meters", lons: []float64{37.61, 37.61158}},
		{name: "just above 100 meters", lons: []float64{37.61, 37.61162}, issue: "INVALID_INPUT"},
		{name: "distant pair", lons: []float64{37.61, 37.62}, issue: "INVALID_INPUT"},
		{name: "three nearby points", lons: []float64{37.61, 37.6101, 37.6102}, issue: "INVALID_INPUT"},
		{name: "nearby wrong corpus and building", lons: []float64{37.61, 37.6101}, houses: []string{"5 к2", "5 с1"}, issue: "GEO_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
				var features []any
				for i, lon := range tc.lons {
					house := "5 корпус 1"
					if len(tc.houses) > 0 {
						house = tc.houses[i]
					}
					features = append(features, photonFeature(lon, "улица Мира", house))
				}
				photonReply(w, features...)
			})
			address := "Москва, ул. Мира, 5 / 1"
			got, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "one", Address: address}}})
			if err != nil || len(got.Items) != 1 {
				t.Fatalf("result=%+v err=%v", got, err)
			}
			item := got.Items[0]
			if len(item.Candidates) != len(tc.lons) {
				t.Fatalf("lost distinct candidates: %+v", item)
			}
			if tc.issue != "" {
				if item.Location != nil || item.Issue == nil || item.Issue.Code != tc.issue {
					t.Fatalf("expected %s without location: %+v", tc.issue, item)
				}
				return
			}
			if item.Issue != nil || item.Location == nil {
				t.Fatalf("expected resolved first point: %+v", item)
			}
			if *item.Location != (c.Location{ID: "one", Address: address, Point: c.Point{Lat: 55.76, Lon: tc.lons[0]}}) {
				t.Fatalf("provider order or original input lost: %+v", item.Location)
			}
		})
	}
}

func TestSlashCorpusSearchFallback(t *testing.T) {
	var calls atomic.Int32
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			if r.URL.Query().Get("q") != "москва улица мира 5 к1" {
				t.Errorf("slash was not normalized for search: %s", r.URL)
			}
			photonReply(w, photonFeature(37.61, "улица Мира", "5"))
			return
		}
		if r.URL.Path != "/structured" || r.URL.Query().Get("housenumber") != "5" {
			t.Errorf("expected structured search by base house: %s", r.URL)
		}
		photonReply(w, photonFeature(37.61, "улица Мира", "5 с1"), photonFeature(37.6101, "улица Мира", "5 к2"), photonFeature(37.6102, "улица Мира", "5 к1"))
	})
	address := "Москва, ул. Мира, д. 5/1, кв. 2"
	got, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "one", Address: address}}})
	if err != nil || len(got.Items) != 1 {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	item := got.Items[0]
	if calls.Load() != 2 || item.Issue != nil || item.Location == nil || item.Location.Point.Lon != 37.6102 || item.Location.Address != address {
		t.Fatalf("calls=%d result=%+v", calls.Load(), item)
	}
}

func TestFractionalHouseWithExplicitCorpusSearch(t *testing.T) {
	for _, structured := range []bool{false, true} {
		t.Run(map[bool]string{false: "full text", true: "structured fallback"}[structured], func(t *testing.T) {
			var calls atomic.Int32
			p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/api/" {
					if r.URL.Query().Get("q") != "москва проспект рязанский 83/2 к2" {
						t.Errorf("fraction or explicit corpus lost in query: %s", r.URL.Query().Get("q"))
					}
					if structured {
						photonReply(w)
						return
					}
				} else if r.URL.Path != "/structured" || r.URL.Query().Get("housenumber") != "83" {
					t.Errorf("unexpected fallback: %s", r.URL)
				}
				// The returned house spelling is the actual Photon / OSM value.
				photonReply(w,
					photonFeature(37.79, "Рязанский проспект", "83 к2"),
					photonFeature(37.79, "Рязанский проспект", "83/2 к1"),
					photonFeature(37.7982718, "Рязанский проспект", "83/2 к2"),
				)
			})
			address := "Город Москва, пр-кт.Рязанский, д. 83/2 к 2"
			got, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "one", Address: address}}})
			if err != nil || len(got.Items) != 1 {
				t.Fatalf("result=%+v err=%v", got, err)
			}
			item := got.Items[0]
			if item.Issue != nil || item.Location == nil || item.Location.Point.Lon != 37.7982718 || item.Location.Address != address || len(item.Candidates) != 1 {
				t.Fatalf("fractional house did not resolve exactly: %+v", item)
			}
			wantCalls := int32(1)
			if structured {
				wantCalls++
			}
			if calls.Load() != wantCalls {
				t.Fatalf("calls=%d want=%d", calls.Load(), wantCalls)
			}
		})
	}
}

func TestGeocodeBatchDeduplicatesInputsWithoutPersistentCache(t *testing.T) {
	var calls atomic.Int32
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		photonReply(w, photonFeature(37.61, "улица Петровка", "2"))
	})
	got, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "one", Address: "Москва, Петровка, 2"}, {ID: "two", Address: "Москва, Петровка, 2"}}})
	if err != nil || calls.Load() != 1 || len(got.Items) != 2 || got.Items[0].Location.ID != "one" || got.Items[1].Location.ID != "two" {
		t.Fatalf("calls=%d result=%+v err=%v", calls.Load(), got, err)
	}
}
func TestSlowRoutingDoesNotBlockAddressSearch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	route := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "NoRoute"})
	}))
	defer route.Close()
	photon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		photonReply(w, photonFeature(37.61, "улица Петровка", "2"))
	}))
	defer photon.Close()
	p, err := NewOSMProvider(OSMOptions{CarURL: route.URL, PhotonURL: photon.URL})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { var response any; done <- p.get(context.Background(), route.URL, &response) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = p.Geocode(ctx, "Москва, Петровка, 2")
	close(release)
	if routeErr := <-done; routeErr != nil {
		t.Fatal(routeErr)
	}
	if err != nil {
		t.Fatalf("address search was blocked by routing: %v", err)
	}
}
func TestUnavailableSearchPreservesEveryAddress(t *testing.T) {
	var calls atomic.Int32
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	got, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "one", Address: "Москва, Петровка, 2"}, {ID: "two", Address: "Москва, Тверская, 1"}}})
	if err != nil || len(got.Items) != 2 || calls.Load() != 3 {
		t.Fatalf("calls=%d result=%+v err=%v", calls.Load(), got, err)
	}
	for _, item := range got.Items {
		if item.Issue == nil || item.Location != nil {
			t.Fatalf("lost input: %+v", item)
		}
	}
}
