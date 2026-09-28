package geo

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func TestAddressComponentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, input, city, street, house string
		want                             bool
	}{
		{"oblast suffix street", "МО, г. Кашира Центральная ул. д. 21", "Кашира", "Центральная улица", "21", true},
		{"oblast fraction", "МО, г. Кашира Кржижановского ул. д. 5/1", "Кашира", "улица Кржижановского", "5/1", true},
		{"capital without commas", "Москва Бирюлевская ул. д. 44", "Москва", "Бирюлёвская улица", "44", true},
		{"drive abbreviation", "Москва Булатниковский пр-зд. д. 6к1", "Москва", "Булатниковский проезд", "6 к1", true},
		{"oblast wrong city", "МО, г. Кашира Центральная ул. д. 21", "Москва", "Центральная улица", "21", false},
		{"omitted type Dmitrovka", "Москва, Большая Дмитровка, 6", "Москва", "улица Большая Дмитровка", "6", true},
		{"omitted type Petrovka", "Москва, Петровка, 2", "Москва", "улица Петровка", "2", true},
		{"case and abbreviation", " Г. МОСКВА, УЛ. ПЕТРОВКА, Д. 2 ", "Москва", "улица Петровка", "2", true},
		{"yo", "Москва, ул. Бирюлёвская, 1 к2", "Москва", "Бирюлевская улица", "1 корпус 2", true},
		{"house commas", "Москва, улица Юных Ленинцев, дом 83, корпус 4", "Москва", "улица Юных Ленинцев", "83 к4", true},
		{"suffixes", "Москва, ул. Мира, д. 1 к2 с3", "Москва", "улица Мира", "1 корпус 2 строение 3", true},
		{"letter", "Москва, ул. Мира, 1А", "Москва", "улица Мира", "1а", true},
		{"fraction", "Москва, ул. Мира, 1/2", "Москва", "улица Мира", "1/2", true},
		{"avenue abbreviation", "Москва, пр-т Мира, 10", "Москва", "проспект Мира", "10", true},
		{"wrong street type", "Москва, переулок Петровка, 2", "Москва", "улица Петровка", "2", false},
		{"missing provider type", "Москва, улица Петровка, 2", "Москва", "Петровка", "2", false},
		{"wrong city", "Москва, Петровка, 2", "Оренбург", "улица Петровка", "2", false},
		{"wrong house", "Москва, Петровка, 2", "Москва", "улица Петровка", "20", false},
		{"extra building", "Москва, Петровка, 2", "Москва", "улица Петровка", "2 с1", false},
		{"missing building", "Москва, Петровка, 2 с1", "Москва", "улица Петровка", "2", false},
		{"corpus is not building", "Москва, Петровка, 2 к1", "Москва", "улица Петровка", "2 с1", false},
		{"ordinal matters", "Москва, улица Тверская, 1", "Москва", "1-я Тверская-Ямская улица", "1", false},
		{"street centroid", "Москва, Петровка, 2", "Москва", "улица Петровка", "", false},
		{"typo needs confirmation", "Москва, Петрофка, 2", "Москва", "улица Петровка", "2", false},
		{"no city", "улица Петровка, 2", "Москва", "улица Петровка", "2", false},
		{"no house", "Москва, Петровка", "Москва", "улица Петровка", "2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := addressMatches(tc.input, parseRussianAddress(tc.input), tc.city, tc.street, tc.house)
			if got != tc.want {
				t.Fatalf("match=%v want=%v parsed=%+v", got, tc.want, parseRussianAddress(tc.input))
			}
		})
	}
}

func photonFeature(lon float64, street, house string) any {
	return map[string]any{"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": []float64{lon, 55.76}}, "properties": map[string]any{"city": "Москва", "street": street, "housenumber": house}}
}
func photonReply(w http.ResponseWriter, features ...any) {
	if features == nil {
		features = []any{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "FeatureCollection", "features": features})
}
func testPhoton(t *testing.T, handler http.HandlerFunc) *OSMProvider {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	p, err := NewOSMProvider(OSMOptions{PhotonURL: s.URL, MinInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPhotonQueryFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name             string
		structuredStatus int
		successAt        int
		wantCalls        int
	}{
		{"first query", 200, 1, 1}, {"structured", 200, 2, 2}, {"original text", 200, 3, 3},
		{"structured unsupported", 404, 3, 3}, {"structured rejects input", 400, 3, 3}, {"no match", 200, 0, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				q := r.URL.Query()
				if q.Get("countrycode") != "RU" || q.Get("limit") != "50" || q.Has("lang") {
					t.Errorf("query=%v", q)
				}
				if len(paths) == 1 && q.Get("q") != "москва петровка 2" {
					t.Errorf("normalized query=%v", q)
				}
				if len(paths) == 2 {
					if r.URL.Path != "/structured" || q.Get("city") != "москва" || q.Get("street") != "петровка" || q.Get("housenumber") != "2" {
						t.Errorf("structured query=%v", r.URL)
					}
					if tc.structuredStatus != 200 {
						w.WriteHeader(tc.structuredStatus)
						return
					}
				}
				if len(paths) == 3 && q.Get("q") != "Москва, Петровка, 2" {
					t.Errorf("original query=%v", q)
				}
				if len(paths) == tc.successAt {
					photonReply(w, photonFeature(37.61, "улица Петровка", "2"))
				} else {
					photonReply(w, photonFeature(37.60, "улица Петровка", "20"))
				}
			})
			points, err := p.Geocode(context.Background(), "Москва, Петровка, 2")
			if err != nil || len(paths) != tc.wantCalls {
				t.Fatalf("points=%v err=%v paths=%v", points, err, paths)
			}
			if (len(points) == 1) != (tc.successAt > 0) {
				t.Fatalf("points=%v", points)
			}
		})
	}
}

func TestPhotonPreservesAmbiguityAndDeduplicatesPoints(t *testing.T) {
	calls := 0
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		photonReply(w,
			photonFeature(37.61, "улица Мира", "2"), photonFeature(37.61, "улица Мира", "2"),
			photonFeature(37.62, "проспект Мира", "2"), photonFeature(37.63, "улица Мира", "20"))
	})
	points, err := p.Geocode(context.Background(), "Москва, Мира, 2")
	if err != nil || len(points) != 2 || calls != 1 {
		t.Fatalf("points=%v err=%v calls=%d", points, err, calls)
	}
	result, err := NewGeoService(p).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "ambiguous", Address: "Москва, Мира, 2"}}})
	if err != nil || result.Items[0].Location != nil || !strings.Contains(result.Items[0].Issue.Message, "неоднозначен") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	points, err = p.Geocode(context.Background(), "Москва, улица Мира, 2")
	if err != nil || len(points) != 1 || points[0].Point.Lon != 37.61 {
		t.Fatalf("explicit type: %v %v", points, err)
	}
}

func TestPhotonCompoundHouseNeverDropsSuffix(t *testing.T) {
	var paths []string
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if !strings.Contains(r.URL.Query().Get("q")+r.URL.Query().Get("housenumber"), "83") {
			t.Error(r.URL)
		}
		photonReply(w, photonFeature(37.6, "улица Юных Ленинцев", "83 к4"))
	})
	points, err := p.Geocode(context.Background(), "Москва, ул. Юных Ленинцев, д. 83с4")
	if err != nil || len(points) != 0 || !reflect.DeepEqual(paths, []string{"/api/", "/structured", "/api/"}) {
		t.Fatalf("points=%v err=%v paths=%v", points, err, paths)
	}
}

func TestPhotonUnusualInputReachesFullTextSearch(t *testing.T) {
	calls := 0
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) { calls++; photonReply(w) })
	for _, address := range []string{"Москва", "Москва, улица Петровка", "улица Петровка, 2"} {
		points, err := p.Geocode(context.Background(), address)
		if err != nil || len(points) != 0 {
			t.Fatalf("%q: %v %v", address, points, err)
		}
	}
	if calls != 3 {
		t.Fatalf("unparsed inputs never reached search: %d", calls)
	}
	_, err := p.Geocode(context.Background(), " ")
	var input *geocodeInputError
	if !errors.As(err, &input) || calls != 3 {
		t.Fatal("blank input must be rejected locally")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Geocode(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestGeocodeRejectsInvalidProvidedCoordinates(t *testing.T) {
	provider := testPhoton(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	for _, point := range []c.Point{{Lat: 91, Lon: 37}, {Lat: 55, Lon: 181}, {Lat: math.NaN(), Lon: 37}, {Lat: 55, Lon: math.Inf(1)}} {
		result, err := NewGeoService(provider).Geocode(context.Background(), c.GeocodeRequest{Locations: []c.LocationInput{{ID: "bad", Address: "Москва", Point: &point}}})
		if err != nil || result.Items[0].Location != nil || result.Items[0].Issue.Code != "INVALID_INPUT" {
			t.Fatalf("%+v %v", result, err)
		}
	}
}

func TestPhotonFallbackDoesNotMaskNetworkFailure(t *testing.T) {
	p := testPhoton(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/structured" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		photonReply(w)
	})
	_, err := p.Geocode(context.Background(), "Москва, Петровка, 2")
	var status *osmHTTPError
	if !errors.As(err, &status) || status.status != 403 {
		t.Fatalf("error=%v", err)
	}
}
