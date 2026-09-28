package storage

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

// ResolveOrderAddress restores imported work before the first plan. During the
// day a restored order goes through a proposal, just like newly received work.
func (s *Store) ResolveOrderAddress(ctx context.Context, id, orderID string, rev int64, location c.Location) (c.ScenarioView, error) {
	var view c.ScenarioView
	p := location.Point
	if strings.TrimSpace(location.Address) == "" || math.IsNaN(p.Lat) || math.IsNaN(p.Lon) || math.IsInf(p.Lat, 0) || math.IsInf(p.Lon, 0) || p.Lat < -90 || p.Lat > 90 || p.Lon < -180 || p.Lon > 180 {
		return view, c.NewError("INVALID_INPUT", "Укажите адрес и корректные координаты")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return view, err
	}
	defer tx.Rollback()
	snap, pid, err := current(ctx, tx, id)
	if err != nil {
		return view, err
	}
	if snap.Revision != rev {
		return view, c.NewError("STALE_VERSION", "Ревизия изменилась")
	}
	if pid != nil {
		return view, c.NewError("EVENT_CONFLICT", "Для действующего плана верните заявку через расчёт вариантов")
	}
	if err := blocked(ctx, tx, snap, pid); err != nil {
		return view, err
	}
	var metadata json.RawMessage
	if err = tx.QueryRowContext(ctx, "SELECT metadata FROM imports WHERE scenario_id=$1", id).Scan(&metadata); err != nil {
		return view, err
	}
	data.RestoreUnlocated(&snap, metadata)
	index := -1
	for i, item := range snap.UnlocatedOrders {
		if item.Order.ID == orderID {
			index = i
			break
		}
	}
	if index < 0 {
		return view, c.NewError("NOT_FOUND", "Заявка без координат не найдена")
	}
	order := snap.UnlocatedOrders[index].Order
	location.ID = order.LocationID
	for _, existing := range snap.Locations {
		if existing.ID == location.ID {
			return view, c.NewError("EVENT_CONFLICT", "Координаты заявки уже определены")
		}
	}
	snap.Locations = append(snap.Locations, location)
	snap.Orders = append(snap.Orders, order)
	sort.SliceStable(snap.Orders, func(i, j int) bool { return snap.Orders[i].SourceOrder < snap.Orders[j].SourceOrder })
	snap.UnlocatedOrders = append(snap.UnlocatedOrders[:index], snap.UnlocatedOrders[index+1:]...)
	issues := snap.Issues[:0]
	for _, issue := range snap.Issues {
		if issue.EntityID == nil || *issue.EntityID != location.ID {
			issues = append(issues, issue)
		}
	}
	snap.Issues = issues
	snap.Revision++
	if _, err = tx.ExecContext(ctx, "INSERT INTO snapshots VALUES($1,$2,$3)", id, snap.Revision, encode(snap)); err != nil {
		return view, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE scenarios SET revision=$2 WHERE id=$1", id, snap.Revision); err != nil {
		return view, err
	}
	view = c.ScenarioView{Snapshot: snap, CurrentPlanID: pid}
	return view, tx.Commit()
}
