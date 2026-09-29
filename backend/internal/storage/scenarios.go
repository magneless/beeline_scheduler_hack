package storage

import (
	"context"
	"encoding/json"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

func (s *Store) ListScenarios(ctx context.Context) ([]c.ScenarioSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.body, s.current_plan_id, COALESCE(i.metadata, '{}'::jsonb)
		FROM scenarios s
		JOIN snapshots p ON p.scenario_id = s.id AND p.revision = s.revision
		LEFT JOIN imports i ON i.scenario_id = s.id
		ORDER BY p.body->>'date' DESC, p.body->>'region_id', s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]c.ScenarioSummary, 0)
	for rows.Next() {
		var body, metadata []byte
		var planID *string
		if err := rows.Scan(&body, &planID, &metadata); err != nil {
			return nil, err
		}
		var snap c.Snapshot
		if err := json.Unmarshal(body, &snap); err != nil {
			return nil, err
		}
		data.RestoreUnlocated(&snap, metadata)
		items = append(items, c.ScenarioSummary{
			ScenarioID: snap.ScenarioID, Revision: snap.Revision,
			RegionID: snap.RegionID, Date: snap.Date,
			OrderCount:     len(snap.Orders) + len(snap.UnlocatedOrders),
			UnlocatedCount: len(snap.UnlocatedOrders), CurrentPlanID: planID,
		})
	}
	return items, rows.Err()
}
