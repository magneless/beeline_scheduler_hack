package data_test

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

const header = "Заявка;Тип заявки BK;Тип заявки HD;Начало;Окончание;Адрес\n"
const office = "Адрес офиса;Москва, офис;;;;\n"

type geoFunc func(context.Context, c.GeocodeRequest) (c.GeocodeResult, error)

func (f geoFunc) Geocode(ctx context.Context, r c.GeocodeRequest) (c.GeocodeResult, error) {
	return f(ctx, r)
}
func TestImportNormsRowsAndReorderedGeocoding(t *testing.T) {
	geo := geoFunc(func(ctx context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
		v, e := (testkit.Geocoder{}).Geocode(ctx, in)
		for a, b := 0, len(v.Items)-1; a < b; a, b = a+1, b-1 {
			v.Items[a], v.Items[b] = v.Items[b], v.Items[a]
		}
		return v, e
	})
	imp := data.Importer{Geo: geo}
	raw := header + "1;Глобальная проблема;Авария;17.08.2026 10:00;17.08.2026 12:00;Москва, 1\n2;Подключение;Заявка на подключение;17.08.2026 10:00;17.08.2026 12:00;Москва, 2\n3;Дозаказ;Дозаказ оборудования;17.08.2026 10:00;17.08.2026 12:00;Москва, 3\n4;Локальная заявка;Нет линка;17.08.2026 10:00;17.08.2026 12:00;Москва, 4\n5;Глобальная проблема;Информация;17.08.2026 10:00;17.08.2026 12:00;Москва, 5\n" + office
	s, _, e := imp.Import(context.Background(), strings.NewReader(raw), "east", "2026-08-17")
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Orders) != 4 || len(s.Issues) != 2 || len(s.Engineers) != 0 || len(s.Locations) != 5 {
		t.Fatalf("unexpected counts: %+v", s)
	}
	for n, sec := range []int64{4800, 4200, 1200, 1800} {
		if s.Orders[n].ServiceSec != sec {
			t.Errorf("order %d duration=%d", n, s.Orders[n].ServiceSec)
		}
	}
	if s.Orders[0].Window.Start.Hour() != 7 || s.Orders[0].ReceivedAt.Hour() != 21 {
		t.Fatal("Moscow conversion wrong")
	}
	if *s.Issues[len(s.Issues)-1].SourceRow != 6 {
		t.Fatal("source row lost")
	}
}
func TestOriginalSyntheticDatasets(t *testing.T) {
	imp := data.Importer{Geo: testkit.Geocoder{}, Root: "../../../datasets/original"}
	for _, d := range data.Catalog {
		t.Run(d.ID, func(t *testing.T) {
			s, _, e := imp.Demo(context.Background(), d.ID)
			if e != nil {
				t.Fatal(e)
			}
			if len(s.Orders) == 0 || len(s.Engineers) != 8 || s.OfficeLocationID == "" {
				t.Fatal("empty dataset")
			}
			t.Logf("orders=%d issues=%d", len(s.Orders), len(s.Issues))
		})
	}
}
func TestImportFailures(t *testing.T) {
	base := header + "1;Глобальная проблема;Авария;17.08.2026 10:00;17.08.2026 12:00;Москва, 1\n"
	for _, tt := range []struct{ name, raw, date string }{{"missing office", base, "2026-08-17"}, {"date mismatch", base + office, "2026-08-18"}, {"broken CSV", header + "\"broken", "2026-08-17"}} {
		t.Run(tt.name, func(t *testing.T) {
			imp := data.Importer{Geo: testkit.Geocoder{}}
			if _, _, e := imp.Import(context.Background(), strings.NewReader(tt.raw), "east", tt.date); e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	imp := data.Importer{Geo: geoFunc(func(context.Context, c.GeocodeRequest) (c.GeocodeResult, error) {
		return c.GeocodeResult{}, c.NewError("GEO_UNAVAILABLE", "offline")
	})}
	_, _, e := imp.Import(context.Background(), strings.NewReader(base+office), "east", "2026-08-17")
	var ce *c.ContractError
	if !errors.As(e, &ce) || ce.Code != "GEO_UNAVAILABLE" {
		t.Fatalf("wrong error %v", e)
	}
}

func TestImportResolvesOfficeBeforeOrders(t *testing.T) {
	raw := header + "1;Глобальная проблема;Авария;17.08.2026 10:00;17.08.2026 12:00;Москва, 1\n" + office
	var requests []c.GeocodeRequest
	imp := data.Importer{Geo: geoFunc(func(_ context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
		requests = append(requests, in)
		return c.GeocodeResult{Items: []c.GeocodeResultItem{{LocationID: "east-office", Issue: &c.Issue{Code: "NOT_FOUND", Message: "нет офиса"}}}}, nil
	})}
	_, _, err := imp.Import(context.Background(), strings.NewReader(raw), "east", "2026-08-17")
	var ce *c.ContractError
	if !errors.As(err, &ce) || ce.Code != "INVALID_INPUT" || !strings.Contains(ce.Message, "Москва, офис") {
		t.Fatalf("err=%v", err)
	}
	if len(requests) != 1 || len(requests[0].Locations) != 1 || requests[0].Locations[0].ID != "east-office" {
		t.Fatalf("requests=%+v", requests)
	}
}

func TestImportRejectsMalformedOfficeGeocoderResponse(t *testing.T) {
	raw := header + office
	for _, tc := range []struct {
		name   string
		result c.GeocodeResult
	}{
		{"wrong id", c.GeocodeResult{Items: []c.GeocodeResultItem{{LocationID: "wrong", Location: &c.Location{ID: "wrong", Point: c.Point{Lat: 55, Lon: 37}}}}}},
		{"nil items", c.GeocodeResult{}},
		{"nan point", c.GeocodeResult{Items: []c.GeocodeResultItem{{LocationID: "east-office", Location: &c.Location{ID: "east-office", Point: c.Point{Lat: math.NaN(), Lon: 37}}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			imp := data.Importer{Geo: geoFunc(func(context.Context, c.GeocodeRequest) (c.GeocodeResult, error) { return tc.result, nil })}
			_, _, err := imp.Import(context.Background(), strings.NewReader(raw), "east", "2026-08-17")
			var ce *c.ContractError
			if !errors.As(err, &ce) || ce.Code != "GEO_UNAVAILABLE" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestDemoOfficeOverridesAreRecorded(t *testing.T) {
	imp := data.Importer{Geo: geoFunc(func(_ context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
		items := make([]c.GeocodeResultItem, 0, len(in.Locations))
		for _, l := range in.Locations {
			if strings.HasSuffix(l.ID, "-office") {
				t.Errorf("demo office sent to geocoder: %+v", l)
			}
			items = append(items, c.GeocodeResultItem{LocationID: l.ID, Location: &c.Location{ID: l.ID, Address: l.Address, Point: c.Point{Lat: 55.7, Lon: 37.6}}})
		}
		return c.GeocodeResult{Items: items}, nil
	}), Root: "../../../datasets/original"}
	for _, tc := range []struct {
		id         string
		overridden bool
	}{{"east", true}, {"southeast", true}, {"southcentral", false}} {
		t.Run(tc.id, func(t *testing.T) {
			snap, metaValue, err := imp.Demo(context.Background(), tc.id)
			if err != nil {
				t.Fatal(err)
			}
			meta, ok := metaValue.(data.ImportMetadata)
			if !ok {
				t.Fatalf("metadata type=%T", metaValue)
			}
			if meta.OfficeOverride == nil {
				t.Fatalf("override=%+v", meta.OfficeOverride)
			}
			var original string
			for _, row := range meta.SourceRows {
				if len(row) > 1 && strings.EqualFold(row[0], "Адрес офиса") {
					original = row[1]
				}
			}
			if original == "" || meta.OfficeOverride.Original != original || !strings.HasPrefix(meta.OfficeOverride.Source, "https://www.openstreetmap.org/way/") {
				t.Fatalf("office provenance=%+v original=%q", meta.OfficeOverride, original)
			}
			var officeFound bool
			for _, loc := range snap.Locations {
				if loc.ID == snap.OfficeLocationID {
					officeFound = true
					if loc.Address != meta.OfficeOverride.Resolved || loc.Point != meta.OfficeOverride.Point {
						t.Fatalf("office=%+v metadata=%+v", loc, meta.OfficeOverride)
					}
				}
			}
			if !officeFound {
				t.Fatal("office missing")
			}
			found := false
			for _, issue := range snap.Issues {
				if issue.Code == "DEMO_OFFICE_OVERRIDE" {
					found = true
				}
			}
			if found != tc.overridden {
				t.Fatalf("override issue=%v", found)
			}
		})
	}
}

func TestImportOriginalDemoCSVDoesNotUseOverride(t *testing.T) {
	f, err := os.Open("../../../datasets/original/Восток Синтетические данные.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var officeSeen bool
	imp := data.Importer{Geo: geoFunc(func(_ context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
		items := make([]c.GeocodeResultItem, 0, len(in.Locations))
		for _, l := range in.Locations {
			if l.ID == "east-office" {
				officeSeen = true
				if l.Point != nil || l.Address != "г. Москва, ул Юных Ленинцев, д 83с 4" {
					t.Fatalf("ordinary office modified: %+v", l)
				}
			}
			items = append(items, c.GeocodeResultItem{LocationID: l.ID, Location: &c.Location{ID: l.ID, Address: l.Address, Point: c.Point{Lat: 55.7, Lon: 37.6}}})
		}
		return c.GeocodeResult{Items: items}, nil
	})}
	_, metaValue, err := imp.Import(context.Background(), f, "east", "2026-08-17")
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := metaValue.(data.ImportMetadata)
	if !ok {
		t.Fatalf("metadata type=%T", metaValue)
	}
	if !officeSeen || meta.OfficeOverride != nil {
		t.Fatalf("officeSeen=%v override=%+v", officeSeen, meta.OfficeOverride)
	}
}

func TestImportInvalidOrdersGeocodesOnlyOffice(t *testing.T) {
	raw := header + "1;Неизвестный тип;Неизвестный тип;17.08.2026 10:00;17.08.2026 12:00;Москва, 1\n" + office
	var calls int
	imp := data.Importer{Geo: geoFunc(func(_ context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
		calls++
		if len(in.Locations) != 1 || in.Locations[0].ID != "east-office" {
			t.Fatalf("unexpected request=%+v", in)
		}
		return c.GeocodeResult{Items: []c.GeocodeResultItem{{LocationID: "east-office", Location: &c.Location{ID: "east-office", Address: "Москва, офис", Point: c.Point{Lat: 55, Lon: 37}}}}}, nil
	})}
	snap, _, err := imp.Import(context.Background(), strings.NewReader(raw), "east", "2026-08-17")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if calls != 1 || len(snap.Orders) != 0 {
		t.Fatalf("calls=%d orders=%d", calls, len(snap.Orders))
	}
}
