package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/geo"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

func TestScenarioEndpointsStreamAndSave(t *testing.T) {
	_, store, _ := integrationServer(t)
	h := (&Server{Store: store, Importer: &data.Importer{
		Geo: geo.NewGeoService(&geo.MockRouteProvider{}), Root: "../../../datasets/original",
	}}).Handler()
	for _, upload := range []bool{false, true} {
		t.Run(map[bool]string{false: "demo", true: "csv"}[upload], func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios", strings.NewReader(`{"demo_dataset_id":"east"}`))
			if upload {
				var body bytes.Buffer
				mw := multipart.NewWriter(&body)
				mw.WriteField("region_id", "east")
				mw.WriteField("date", "2026-08-17")
				part, err := mw.CreateFormFile("file", "orders.csv")
				if err != nil {
					t.Fatal(err)
				}
				csv, err := os.Open("../../../datasets/original/Восток Синтетические данные.csv")
				if err != nil {
					t.Fatal(err)
				}
				defer csv.Close()
				if _, err := io.Copy(part, csv); err != nil {
					t.Fatal(err)
				}
				mw.Close()
				r = httptest.NewRequest(http.MethodPost, "/api/v1/scenarios/import", &body)
				r.Header.Set("Content-Type", mw.FormDataContentType())
			}
			r.Header.Set("Accept", "text/event-stream")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") {
				t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
			}
			var view c.ScenarioView
			completed, total := -1, 0
			saving := false
			for _, event := range strings.Split(w.Body.String(), "\n\n") {
				kind, payload, ok := strings.Cut(event, "\ndata: ")
				if !ok {
					continue
				}
				switch kind {
				case "event: progress":
					var update progress.Update
					if err := json.Unmarshal([]byte(payload), &update); err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(update.Stage, "geocoding") {
						if update.Total <= 1 || update.Completed != completed+1 || (total > 0 && update.Total != total) {
							t.Fatalf("invalid address progress after %d/%d: %+v", completed, total, update)
						}
						completed, total = update.Completed, update.Total
					}
					if update.Stage == "saving" {
						saving = true
					}
				case "event: result":
					if !saving {
						t.Fatal("result arrived before saving stage")
					}
					if err := json.Unmarshal([]byte(payload), &view); err != nil {
						t.Fatal(err)
					}
				case "event: error":
					t.Fatalf("unexpected stream error: %s", payload)
				}
			}
			if total == 0 || completed != total || len(view.Snapshot.Orders) != total || view.Snapshot.ScenarioID == "" {
				t.Fatalf("incomplete scenario: %d/%d, orders=%d id=%q", completed, total, len(view.Snapshot.Orders), view.Snapshot.ScenarioID)
			}
			saved, err := store.GetScenario(context.Background(), view.Snapshot.ScenarioID, 0)
			if err != nil || len(saved.Snapshot.Orders) != total {
				t.Fatalf("scenario was not saved: %v", err)
			}
		})
	}
}

func TestScenarioStreamErrorKeepsJSONCompatibility(t *testing.T) {
	h := (&Server{Importer: &data.Importer{}}).Handler()
	for _, accept := range []string{"application/json", "text/event-stream"} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios", strings.NewReader(`{"demo_dataset_id":"missing"}`))
		r.Header.Set("Accept", accept)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if accept == "application/json" && (w.Code != 404 || strings.Contains(w.Body.String(), "event:")) {
			t.Fatalf("JSON error changed: %d %s", w.Code, w.Body.String())
		}
		if accept == "text/event-stream" && (w.Code != 200 || !strings.Contains(w.Body.String(), "event: error")) {
			t.Fatalf("missing streamed error: %d %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"code":"NOT_FOUND"`) || strings.Contains(w.Body.String(), "event: result") {
			t.Fatalf("incorrect error: %s", w.Body.String())
		}
	}
}
