package data_test

import (
	"context"
	"errors"
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
