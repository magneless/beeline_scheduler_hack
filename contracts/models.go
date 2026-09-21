package contracts

import "time"

type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Location struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Point   Point  `json:"point"`
}

type LocationInput struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Point   *Point `json:"point"`
}

type Order struct {
	ID                string      `json:"id"`
	LocationID        string      `json:"location_id"`
	RequiredSkills    []string    `json:"required_skills"`
	RequiredTransport *Transport  `json:"required_transport"`
	Window            Window      `json:"window"`
	ServiceSec        int64       `json:"service_sec"`
	Priority          Priority    `json:"priority"`
	SourceOrder       int64       `json:"source_order"`
	Status            OrderStatus `json:"status"`
}

type Engineer struct {
	ID          string    `json:"id"`
	Skills      []string  `json:"skills"`
	Transport   Transport `json:"transport"`
	Shift       Window    `json:"shift"`
	Available   bool      `json:"available"`
	SourceOrder int64     `json:"source_order"`
}

type Issue struct {
	SourceRow *int    `json:"source_row"`
	EntityID  *string `json:"entity_id"`
	Field     *string `json:"field"`
	Code      string  `json:"code"`
	Message   string  `json:"message"`
}

type Snapshot struct {
	ScenarioID       string     `json:"scenario_id"`
	Revision         int64      `json:"revision"`
	RegionID         string     `json:"region_id"`
	Date             string     `json:"date"`
	Timezone         string     `json:"timezone"`
	OfficeLocationID string     `json:"office_location_id"`
	Locations        []Location `json:"locations"`
	Orders           []Order    `json:"orders"`
	Engineers        []Engineer `json:"engineers"`
	Issues           []Issue    `json:"issues"`
}

type ScenarioView struct {
	Snapshot      Snapshot `json:"snapshot"`
	CurrentPlanID *string  `json:"current_plan_id"`
}

type EngineerState struct {
	EngineerID      string    `json:"engineer_id"`
	StartLocationID string    `json:"start_location_id"`
	AvailableFrom   time.Time `json:"available_from"`
}

type Visit struct {
	OrderID   string    `json:"order_id"`
	ArrivalAt time.Time `json:"arrival_at"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
}

type Leg struct {
	ID             string    `json:"id"`
	FromLocationID string    `json:"from_location_id"`
	ToLocationID   string    `json:"to_location_id"`
	StartAt        time.Time `json:"start_at"`
	EndAt          time.Time `json:"end_at"`
	DistanceM      int64     `json:"distance_m"`
	GeoContextID   string    `json:"geo_context_id"`
	Geometry       []Point   `json:"geometry"`
}

type Route struct {
	EngineerID      string    `json:"engineer_id"`
	StartLocationID string    `json:"start_location_id"`
	StartAt         time.Time `json:"start_at"`
	Visits          []Visit   `json:"visits"`
	Legs            []Leg     `json:"legs"`
}

type UnassignedOrder struct {
	OrderID    string `json:"order_id"`
	ReasonCode string `json:"reason_code"`
	Message    string `json:"message"`
}

type EngineerDistance struct {
	EngineerID string `json:"engineer_id"`
	DistanceM  int64  `json:"distance_m"`
}

type Metrics struct {
	AssignedCount     int                `json:"assigned_count"`
	UnassignedCount   int                `json:"unassigned_count"`
	UsedEngineerCount int                `json:"used_engineer_count"`
	TotalDistanceM    int64              `json:"total_distance_m"`
	PerEngineer       []EngineerDistance `json:"per_engineer"`
}

type Assignment struct {
	EngineerID string    `json:"engineer_id"`
	Sequence   int       `json:"sequence"`
	ArrivalAt  time.Time `json:"arrival_at"`
	StartAt    time.Time `json:"start_at"`
	EndAt      time.Time `json:"end_at"`
}

type PlanChange struct {
	OrderID string           `json:"order_id"`
	Before  *Assignment      `json:"before"`
	After   *Assignment      `json:"after"`
	Reason  PlanChangeReason `json:"reason"`
}

type PlanDraft struct {
	ScenarioID        string            `json:"scenario_id"`
	SnapshotRevision  int64             `json:"snapshot_revision"`
	BasePlanID        *string           `json:"base_plan_id"`
	AsOf              time.Time         `json:"as_of"`
	Routes            []Route           `json:"routes"`
	Unassigned        []UnassignedOrder `json:"unassigned"`
	CancelledOrderIDs []string          `json:"cancelled_order_ids"`
	Issues            []Issue           `json:"issues"`
	Metrics           Metrics           `json:"metrics"`
	BaselineMetrics   *Metrics          `json:"baseline_metrics"`
	Changes           []PlanChange      `json:"changes"`
	Termination       Termination       `json:"termination"`
}

type PlanResult struct {
	Draft          PlanDraft `json:"draft"`
	TargetSnapshot Snapshot  `json:"target_snapshot"`
	AppliedEvent   *Event    `json:"applied_event"`
}

type Plan struct {
	ID string `json:"id"`
	PlanDraft
}

type Event struct {
	ID         string       `json:"id"`
	OccurredAt time.Time    `json:"occurred_at"`
	Type       EventType    `json:"type"`
	Payload    EventPayload `json:"payload"`
}

type EventPayload struct {
	Order      *Order         `json:"order,omitempty"`
	Location   *LocationInput `json:"location,omitempty"`
	OrderID    string         `json:"order_id,omitempty"`
	EngineerID string         `json:"engineer_id,omitempty"`
}
