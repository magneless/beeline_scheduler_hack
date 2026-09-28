package data

import (
	"encoding/json"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// Older imports kept rejected addresses only in metadata. Restore their
// presentation without changing saved routes or assigning guessed locations.
func RestoreUnlocated(snap *c.Snapshot, metadata []byte) {
	var meta ImportMetadata
	if json.Unmarshal(metadata, &meta) != nil {
		return
	}
	known := map[string]bool{}
	for _, o := range snap.Orders {
		known[o.ID] = true
	}
	for _, o := range snap.UnlocatedOrders {
		known[o.Order.ID] = true
	}
	zone, err := time.LoadLocation(snap.Timezone)
	if err != nil {
		return
	}
	day, err := time.ParseInLocation("2006-01-02", snap.Date, zone)
	if err != nil {
		return
	}
	for _, row := range meta.SourceRows {
		if len(row) < 7 || known[row[0]] {
			continue
		}
		id := row[0]
		lid := snap.RegionID + "-location-" + id
		var failure *c.Issue
		for i := range snap.Issues {
			issue := &snap.Issues[i]
			if issue.EntityID != nil && *issue.EntityID == lid {
				failure = issue
				break
			}
		}
		if failure == nil {
			continue
		}
		work, skill, sec, eq, ok := classify(row[1], row[2])
		start, e1 := time.ParseInLocation("02.01.2006 15:04", row[3], zone)
		end, e2 := time.ParseInLocation("02.01.2006 15:04", row[4], zone)
		if !ok || e1 != nil || e2 != nil || end.Before(start) {
			continue
		}
		priority := c.PriorityNormal
		if work == c.WorkTypeEmergency {
			priority = c.PriorityUrgent
		}
		source := int64(0)
		if failure.SourceRow != nil {
			source = int64(*failure.SourceRow)
		}
		snap.UnlocatedOrders = append(snap.UnlocatedOrders, c.UnlocatedOrder{
			Order:   c.Order{ID: id, LocationID: lid, WorkType: work, RequiredSkills: []string{skill}, Window: c.Window{Start: start.UTC(), End: end.UTC()}, ReceivedAt: day.UTC(), ServiceSec: sec, Priority: priority, EquipmentRequired: eq, SourceOrder: source, Status: c.OrderStatusActive},
			Address: row[6], Message: failure.Message,
		})
		known[id] = true
	}
}
