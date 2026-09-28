package data_test

import (
	"context"
	"encoding/json"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
	"strings"
	"testing"
)

func TestUnresolvedEmergencyRemainsVisibleAndLegacyImportIsRecoverable(t *testing.T) {
	geo := geoFunc(func(ctx context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
		out, err := (testkit.Geocoder{}).Geocode(ctx, in)
		for i := range out.Items {
			if out.Items[i].LocationID == "east-location-1" {
				id := out.Items[i].LocationID
				out.Items[i].Location = nil
				out.Items[i].Issue = &c.Issue{Code: "GEO_UNAVAILABLE", EntityID: &id, Message: "Дом не найден"}
			}
		}
		return out, err
	})
	imp := data.Importer{Geo: geo}
	raw := "Заявка;Тип заявки BK;Тип заявки HD;Начало;Окончание;Район;Адрес\n1;Глобальная проблема;Авария;17.08.2026 10:00;17.08.2026 12:00;Москва;Москва, ул. Мира, 1\nАдрес офиса;Москва, офис;;;;;\n"
	snap, meta, err := imp.Import(context.Background(), strings.NewReader(raw), "east", "2026-08-17")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Orders) != 0 || len(snap.UnlocatedOrders) != 1 || snap.UnlocatedOrders[0].Order.WorkType != c.WorkTypeEmergency || snap.UnlocatedOrders[0].Message != "Дом не найден" {
		t.Fatalf("emergency disappeared: %+v", snap)
	}
	snap.UnlocatedOrders = nil
	encoded, _ := json.Marshal(meta)
	data.RestoreUnlocated(&snap, encoded)
	data.RestoreUnlocated(&snap, encoded)
	if len(snap.UnlocatedOrders) != 1 || snap.UnlocatedOrders[0].Order.ID != "1" {
		t.Fatalf("legacy emergency missing or duplicated: %+v", snap.UnlocatedOrders)
	}
}
