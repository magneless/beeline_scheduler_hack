package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

const setupOrders = "Заявка;Тип заявки BK;Тип заявки HD;Начало;Окончание;Адрес\njob-1;Подключение;Конвергенция абонента;28.09.2026 10:00;28.09.2026 12:00;Москва, улица Юных Ленинцев, 84\n"
const setupEngineers = "id;skills;transport;shift_start;shift_end;available;router;tv_box\ncrew-1;connection;car;10:00;22:00;true;4;2\n"

func daySetupRequest(t *testing.T, orders, engineers, address, point string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("date", "2026-09-28")
	w.WriteField("office_address", address)
	if point != "" {
		w.WriteField("office_point", point)
	}
	for name, content := range map[string]string{"file": orders, "engineers_file": engineers} {
		if content == "" {
			continue
		}
		part, err := w.CreateFormFile(name, name+".csv")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios/import", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func TestDaySetupPersistsOrdersOfficeAndEngineersTogether(t *testing.T) {
	h, _, _ := integrationServer(t)
	for _, point := range []string{"", `{"lat":55.7022013,"lon":37.7739593}`} {
		r := daySetupRequest(t, setupOrders, "\ufeff"+setupEngineers, "Москва, улица Юных Ленинцев, 83 корпус 4", point)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 201 {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
		var created, saved c.ScenarioView
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		call(t, h, "GET", "/scenarios/"+created.Snapshot.ScenarioID, nil, 200, &saved)
		s := saved.Snapshot
		if s.RegionID != "custom" || s.Date != "2026-09-28" || s.Revision != 1 || len(s.Orders) != 1 || len(s.Engineers) != 1 || s.Engineers[0].ID != "crew-1" {
			t.Fatalf("incorrect imported day: %+v", s)
		}
		if err := c.ValidateEngineerShifts(s); err != nil {
			t.Fatal(err)
		}
		for _, issue := range s.Issues {
			if issue.Code == "ENGINEERS_REQUIRED" || issue.Code == "DEMO_ENGINEERS" || issue.Code == "DEMO_OFFICE_OVERRIDE" {
				t.Fatalf("unexpected demo/default: %+v", issue)
			}
		}
		if point != "" {
			found := false
			for _, location := range s.Locations {
				if location.ID == s.OfficeLocationID {
					found = location.Point == (c.Point{Lat: 55.7022013, Lon: 37.7739593})
				}
			}
			if !found {
				t.Fatal("chosen office point was not preserved")
			}
		}
	}
}

type failSetupGeocoder struct{ calls *int }

func (g failSetupGeocoder) Geocode(context.Context, c.GeocodeRequest) (c.GeocodeResult, error) {
	*g.calls++
	return c.GeocodeResult{}, c.NewError("GEO_UNAVAILABLE", "unexpected geocoding")
}

func TestDaySetupRejectsBadRosterOrOfficeBeforeCreatingScenario(t *testing.T) {
	for _, tc := range []struct{ name, engineers, point string }{
		{"missing roster", "", ""},
		{"invalid shift", strings.ReplaceAll(setupEngineers, "22:00", "23:00"), ""},
		{"bad header", "wrong;header\n1;2\n", ""},
		{"invalid coordinates", setupEngineers, `{"lat":95,"lon":37}`},
		{"partial coordinates", setupEngineers, `{"lat":55}`},
		{"null coordinates", setupEngineers, `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := (&Server{Importer: &data.Importer{Geo: failSetupGeocoder{&calls}}}).Handler()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, daySetupRequest(t, setupOrders, tc.engineers, "Москва, офис", tc.point))
			if w.Code != 422 || calls != 0 {
				t.Fatalf("status=%d geocoding=%d body=%s", w.Code, calls, w.Body.String())
			}
		})
	}
}

func TestWorkingCatalogExcludesContractFixtures(t *testing.T) {
	h := (&Server{Importer: &data.Importer{Prepared: map[string]c.Snapshot{"contract-example": {}}}}).Handler()
	var got struct{ Items []data.Dataset }
	call(t, h, "GET", "/demo-datasets", nil, 200, &got)
	if len(got.Items) != len(data.Catalog) {
		t.Fatal(got)
	}
	for _, d := range got.Items {
		if d.ID == "contract-example" {
			t.Fatal("fixture exposed in working catalog")
		}
	}
}
