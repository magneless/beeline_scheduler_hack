package httpapi

import (
	"math"
	"strings"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

func validateNewOrder(e c.Event, p c.UrgentOrderAdded) error {
	o := p.Order
	if strings.TrimSpace(o.ID) == "" || strings.TrimSpace(o.LocationID) == "" || o.Status != c.OrderStatusActive || o.Execution != nil || !o.ReceivedAt.Equal(e.OccurredAt) || o.Window.Start.IsZero() || o.Window.End.IsZero() || o.Window.End.Before(o.Window.Start) || o.Window.Start.Nanosecond() != 0 || o.Window.End.Nanosecond() != 0 || o.RequiredSkills == nil || o.EquipmentRequired == nil || o.ServiceSec <= 0 || o.ServiceSec > math.MaxInt64/int64(time.Second) {
		return invalid("Некорректные параметры новой заявки")
	}
	if e.Type == "urgent_order_added" {
		if o.WorkType != c.WorkTypeEmergency || o.Priority != c.PriorityUrgent || o.ServiceSec != 4800 {
			return invalid("Авария должна иметь срочный приоритет и длительность 4800 секунд")
		}
	} else if o.Priority != c.PriorityNormal || (o.WorkType != c.WorkTypeConnection && o.WorkType != c.WorkTypeRepair && o.WorkType != c.WorkTypeAdditional) {
		return invalid("Некорректный тип или приоритет обычной заявки")
	}
	if o.RequiredTransport != nil && *o.RequiredTransport != c.TransportCar && *o.RequiredTransport != c.TransportWalk {
		return invalid("Некорректный транспорт")
	}
	for _, skill := range o.RequiredSkills {
		if strings.TrimSpace(skill) == "" {
			return invalid("Пустой навык")
		}
	}
	if p.Location != nil {
		if p.Location.ID != o.LocationID {
			return invalid("location.id не совпадает с order.location_id")
		}
		if strings.TrimSpace(p.Location.Address) == "" {
			return invalid("Требуется адрес новой локации")
		}
		if pt := p.Location.Point; pt != nil && (math.IsNaN(pt.Lat) || math.IsNaN(pt.Lon) || math.IsInf(pt.Lat, 0) || math.IsInf(pt.Lon, 0) || pt.Lat < -90 || pt.Lat > 90 || pt.Lon < -180 || pt.Lon > 180) {
			return invalid("Некорректные координаты")
		}
	}
	return equipment(o.EquipmentRequired)
}
