package httpapi

import (
	"encoding/json"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"testing"
	"time"
)

func TestNewOrderValidationAndNormalization(t *testing.T) {
	at := time.Date(2026, 8, 17, 10, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	base := c.UrgentOrderAdded{Order: c.Order{ID: "ordinary-1", LocationID: "new-location", WorkType: c.WorkTypeRepair, Priority: c.PriorityNormal, Status: c.OrderStatusActive, ReceivedAt: at, Window: c.Window{Start: at, End: at.Add(time.Hour)}, ServiceSec: 1800, RequiredSkills: []string{"repair"}, EquipmentRequired: map[c.Equipment]int64{}}, Location: &c.LocationInput{ID: "new-location", Address: "Москва, Тверская улица, 13"}}
	cases := []struct {
		name   string
		mutate func(*c.UrgentOrderAdded)
		valid  bool
	}{
		{"valid", func(*c.UrgentOrderAdded) {}, true},
		{"existing location", func(p *c.UrgentOrderAdded) { p.Location = nil }, true},
		{"invalid transport", func(p *c.UrgentOrderAdded) { v := c.Transport("bike"); p.Order.RequiredTransport = &v }, false},
		{"empty skill", func(p *c.UrgentOrderAdded) { p.Order.RequiredSkills = []string{" "} }, false},
		{"empty address", func(p *c.UrgentOrderAdded) { p.Location.Address = " " }, false},
		{"bad coordinates", func(p *c.UrgentOrderAdded) { p.Location.Point = &c.Point{Lat: 91, Lon: 30} }, false},
		{"location mismatch", func(p *c.UrgentOrderAdded) { p.Location.ID = "other" }, false},
		{"zero duration", func(p *c.UrgentOrderAdded) { p.Order.ServiceSec = 0 }, false},
		{"duration overflow", func(p *c.UrgentOrderAdded) { p.Order.ServiceSec = 1 << 62 }, false},
		{"negative stock", func(p *c.UrgentOrderAdded) { p.Order.EquipmentRequired[c.EquipmentRouter] = -1 }, false},
		{"wrong priority", func(p *c.UrgentOrderAdded) { p.Order.Priority = c.PriorityUrgent }, false},
		{"emergency as ordinary", func(p *c.UrgentOrderAdded) { p.Order.WorkType = c.WorkTypeEmergency }, false},
		{"received mismatch", func(p *c.UrgentOrderAdded) { p.Order.ReceivedAt = at.Add(time.Second) }, false},
		{"fractional window", func(p *c.UrgentOrderAdded) { p.Order.Window.Start = at.Add(time.Nanosecond) }, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(base)
			var p c.UrgentOrderAdded
			json.Unmarshal(raw, &p)
			tt.mutate(&p)
			raw, _ = json.Marshal(p)
			event := c.Event{ID: "event", Type: "ordinary_order_added", OccurredAt: at, Payload: raw}
			err := validateEvent(event)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if tt.valid {
				normalized := normalizeEvent(event)
				json.Unmarshal(normalized.Payload, &p)
				if p.Order.ReceivedAt.Location() != time.UTC || !p.Order.ReceivedAt.Equal(at) {
					t.Fatal("normalization changed event instant")
				}
			}
		})
	}
	base.Order.WorkType = c.WorkTypeEmergency
	base.Order.Priority = c.PriorityUrgent
	base.Order.ServiceSec = 4800
	raw, _ := json.Marshal(base)
	if err := validateEvent(c.Event{ID: "urgent", Type: "urgent_order_added", OccurredAt: at, Payload: raw}); err != nil {
		t.Fatal(err)
	}
}
