package contracts

import "time"

type BuildPlanRequest struct {
	RequestID             string  `json:"request_id"`
	ScenarioID            string  `json:"scenario_id"`
	SnapshotRevision      int64   `json:"snapshot_revision"`
	ExpectedCurrentPlanID *string `json:"expected_current_plan_id"`
}

type ReplanRequest struct {
	RequestID        string `json:"request_id"`
	ScenarioID       string `json:"scenario_id"`
	SnapshotRevision int64  `json:"snapshot_revision"`
	BasePlanID       string `json:"base_plan_id"`
	Event            Event  `json:"event"`
}

type PlanCommit struct {
	RequestID             string     `json:"request_id"`
	ExpectedRevision      int64      `json:"expected_revision"`
	ExpectedCurrentPlanID *string    `json:"expected_current_plan_id"`
	Result                PlanResult `json:"result"`
}

type Run struct {
	ID         string         `json:"id"`
	ScenarioID string         `json:"scenario_id"`
	Status     RunStatus      `json:"status"`
	PlanID     *string        `json:"plan_id"`
	Error      *ContractError `json:"error"`
}

type GeocodeRequest struct {
	RegionID  string          `json:"region_id"`
	Locations []LocationInput `json:"locations"`
}

type GeocodeItem struct {
	LocationID string    `json:"location_id"`
	Location   *Location `json:"location"`
	Issue      *Issue    `json:"issue"`
}

type GeocodeResult struct {
	Items []GeocodeItem `json:"items"`
}

type MatrixRequest struct {
	Locations    []Location  `json:"locations"`
	Profiles     []Transport `json:"profiles"`
	GeoContextID *string     `json:"geo_context_id"`
}

type RouteLegRequest struct {
	LegID          string    `json:"leg_id"`
	FromLocationID string    `json:"from_location_id"`
	ToLocationID   string    `json:"to_location_id"`
	Profile        Transport `json:"profile"`
}

type RoutesRequest struct {
	GeoContextID string            `json:"geo_context_id"`
	Locations    []Location        `json:"locations"`
	Legs         []RouteLegRequest `json:"legs"`
}

type RouteGeometry struct {
	LegID    string  `json:"leg_id"`
	Geometry []Point `json:"geometry"`
}

type RoutesGeometry struct {
	Items []RouteGeometry `json:"items"`
}

type PositionRequest struct {
	Leg Leg       `json:"leg"`
	At  time.Time `json:"at"`
}

type PositionResult struct {
	Point              Point   `json:"point"`
	ElapsedDurationSec int64   `json:"elapsed_duration_sec"`
	ElapsedDistanceM   int64   `json:"elapsed_distance_m"`
	ElapsedGeometry    []Point `json:"elapsed_geometry"`
}
