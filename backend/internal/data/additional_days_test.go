package data_test

import (
	"context"
	"strings"
	"testing"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

func TestAdditionalDaysImportSourceRowsAndEmergencies(t *testing.T) {
	imp := data.Importer{Geo: testkit.Geocoder{}, Root: "../../../datasets/original"}
	for _, tc := range []struct {
		id                        string
		rows, orders, emergencies int
	}{
		{"east-2026-09-28", 74, 71, 2},
		{"east-2026-09-29", 78, 75, 4},
		{"southeast-2026-09-28", 102, 100, 3},
		{"southeast-2026-09-29", 87, 82, 3},
		{"southcentral-2026-09-28", 79, 77, 5},
		{"southcentral-2026-09-29", 77, 74, 2},
	} {
		t.Run(tc.id, func(t *testing.T) {
			snap, rawMeta, err := imp.Demo(context.Background(), tc.id)
			if err != nil {
				t.Fatal(err)
			}
			meta := rawMeta.(data.ImportMetadata)
			if len(meta.SourceRows) != tc.rows || len(snap.Orders) != tc.orders {
				t.Fatalf("rows=%d orders=%d issues=%+v", len(meta.SourceRows), len(snap.Orders), snap.Issues)
			}
			if meta.OfficeOverride == nil || meta.OfficeOverride.Original != "" || meta.OfficeOverride.Source == "" {
				t.Fatalf("missing office provenance: %+v", meta.OfficeOverride)
			}
			invalidRows, officeNotice, emergencies := 0, 0, 0
			for _, issue := range snap.Issues {
				if issue.Code == "DEMO_OFFICE_OVERRIDE" && strings.Contains(issue.Message, "нет адреса офиса") {
					officeNotice++
				}
				if issue.Code == "INVALID_INPUT" {
					invalidRows++
					if issue.SourceRow == nil || issue.Field == nil || *issue.Field != "work_type" {
						t.Fatalf("unexpected rejected row: %+v", issue)
					}
					row := meta.SourceRows[*issue.SourceRow-2]
					if row[3] != "Информация" {
						t.Fatalf("work order lost: %+v", row)
					}
				}
			}
			for _, order := range snap.Orders {
				if order.WorkType == c.WorkTypeEmergency {
					emergencies++
				}
				if order.Status != c.OrderStatusActive {
					t.Fatalf("historical status leaked into day replay: %+v", order)
				}
			}
			if invalidRows+tc.orders != tc.rows || officeNotice != 1 || emergencies != tc.emergencies {
				t.Fatalf("rejected=%d office notices=%d emergencies=%d", invalidRows, officeNotice, emergencies)
			}
		})
	}
}

func TestAdditionalDayRepairLabelsPreserveEquipment(t *testing.T) {
	for _, label := range []string{
		"ТВ. Рассыпание/замирание картинки",
		"TVE/ENT. Проблема с качеством изображения/звука",
		"TVE/ENT. Замирание картинки/появляется круг загрузки",
		"Роутер. Не выходит ONLINE",
		"TVE/ENT/Яндекс.ТВ. Замена приставки техником",
	} {
		t.Run(label, func(t *testing.T) {
			imp := data.Importer{Geo: testkit.Geocoder{}}
			raw := header + "1;Локальная заявка;" + label + ";17.08.2026 10:00;17.08.2026 12:00;Москва, 1\n" + office
			snap, _, err := imp.Import(context.Background(), strings.NewReader(raw), "east", "2026-08-17")
			if err != nil || len(snap.Orders) != 1 {
				t.Fatalf("err=%v orders=%+v", err, snap.Orders)
			}
			order := snap.Orders[0]
			if order.WorkType != c.WorkTypeRepair || order.ServiceSec != 1800 {
				t.Fatalf("unexpected repair norm: %+v", order)
			}
			if strings.Contains(label, "Замена приставки") {
				if len(order.EquipmentRequired) != 1 || order.EquipmentRequired[c.EquipmentTVBox] != 1 {
					t.Fatalf("replacement requires one TV box: %+v", order)
				}
			} else if len(order.EquipmentRequired) != 0 {
				t.Fatalf("diagnostics must not imply replacement: %+v", order)
			}
		})
	}
}
