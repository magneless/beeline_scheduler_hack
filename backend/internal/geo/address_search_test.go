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
		if !parts.ok || parts.city != "домодедово" || parts.street != "улица жуковского" || parts.house != "14/18" {
			t.Fatalf("%s: %+v", address, parts)
		}
	}
}

func TestUnusualAddressMatchesCompleteCandidate(t *testing.T) {
	for _, tc := range []struct{ address, city, street, house string }{
		{"Город Москва, б-р.Самаркандский Квартал 137а, д. к5", "Москва", "квартал Самаркандский Бульвар 137А", "к5"},
		{"г. Великий Новгород Большая Московская ул. д. 5", "Великий Новгород", "Большая Московская улица", "5"},
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
