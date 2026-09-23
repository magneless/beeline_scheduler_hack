package httpapi

import (
	"encoding/json"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// Called after validation; normalize timestamps before persisting or comparing commands.
func normalizeEvent(e c.Event) c.Event {
	var value any
	switch e.Type {
	case "urgent_order_added":
		var p c.UrgentOrderAdded
		json.Unmarshal(e.Payload, &p)
		p.Order.Window.Start = p.Order.Window.Start.UTC()
		p.Order.Window.End = p.Order.Window.End.UTC()
		p.Order.ReceivedAt = p.Order.ReceivedAt.UTC()
		value = p
	case "order_status_changed":
		var p c.OrderStatusChanged
		json.Unmarshal(e.Payload, &p)
		if p.ExpectedEndAt != nil {
			utc := p.ExpectedEndAt.UTC()
			p.ExpectedEndAt = &utc
		}
		value = p
	case "order_cancelled":
		var p c.OrderCancelled
		json.Unmarshal(e.Payload, &p)
		value = p
	case "engineer_unavailable":
		var p c.EngineerUnavailable
		json.Unmarshal(e.Payload, &p)
		value = p
	}
	if value != nil {
		e.Payload, _ = json.Marshal(value)
	}
	return e
}
