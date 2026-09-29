package httpapi

import (
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

func TestAdditionalDayScenariosAreListedAndPersisted(t *testing.T) {
	h, _, _ := integrationServer(t)
	var catalog struct{ Items []data.Dataset }
	call(t, h, "GET", "/demo-datasets", nil, 200, &catalog)
	additional := 0
	for _, dataset := range catalog.Items {
		if dataset.Date != "2026-09-28" && dataset.Date != "2026-09-29" {
			continue
		}
		additional++
		t.Run(dataset.ID, func(t *testing.T) {
			var created, saved c.ScenarioView
			call(t, h, "POST", "/scenarios", map[string]string{"demo_dataset_id": dataset.ID}, 201, &created)
			call(t, h, "GET", "/scenarios/"+created.Snapshot.ScenarioID, nil, 200, &saved)
			if saved.Snapshot.Date != dataset.Date || saved.Snapshot.RegionID != dataset.RegionID || len(saved.Snapshot.Orders) == 0 || len(saved.Snapshot.Engineers) != 8 {
				t.Fatalf("incorrect saved scenario: %+v", saved.Snapshot)
			}
			if len(saved.Snapshot.Orders) != len(created.Snapshot.Orders) {
				t.Fatal("orders lost while saving scenario")
			}
		})
	}
	if additional != 6 {
		t.Fatalf("expected six additional days in catalog, got %d", additional)
	}
}
