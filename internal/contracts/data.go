package contracts

import (
	"context"
	"encoding/json"
	"time"
)

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
type EngineerDistance struct {
	EngineerID string `json:"engineer_id"`
	DistanceM  int64  `json:"distance_m"`
}
type Metrics struct {
	AssignedCount     int                `json:"assigned_count"`
	CompletedCount    int                `json:"completed_count"`
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
	OrderID string      `json:"order_id"`
	Before  *Assignment `json:"before"`
	After   *Assignment `json:"after"`
	Reason  string      `json:"reason"`
}
type PlanDraft struct {
	ScenarioID         string                         `json:"scenario_id"`
	SnapshotRevision   int64                          `json:"snapshot_revision"`
	BasePlanID         *string                        `json:"base_plan_id"`
	AsOf               time.Time                      `json:"as_of"`
	Routes             []Route                        `json:"routes"`
	Unassigned         []UnassignedOrder              `json:"unassigned"`
	CancelledOrderIDs  []string                       `json:"cancelled_order_ids"`
	CompletedOrderIDs  []string                       `json:"completed_order_ids"`
	EquipmentRemaining map[string]map[Equipment]int64 `json:"equipment_remaining"`
	Issues             []Issue                        `json:"issues"`
	Metrics            Metrics                        `json:"metrics"`
	BaselineMetrics    *Metrics                       `json:"baseline_metrics"`
	Changes            []PlanChange                   `json:"changes"`
	Termination        Termination                    `json:"termination"`
}
type Plan struct {
	ID string `json:"id"`
	PlanDraft
}
type Event struct {
	ID         string          `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
}
type UrgentOrderAdded struct {
	Order    Order          `json:"order"`
	Location *LocationInput `json:"location"`
}
type OrderCancelled struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
type EngineerUnavailable struct {
	EngineerID string `json:"engineer_id"`
}
type OrderStatusChanged struct {
	OrderID       string      `json:"order_id"`
	Status        OrderStatus `json:"status"`
	EngineerID    string      `json:"engineer_id"`
	ExpectedEndAt *time.Time  `json:"expected_end_at"`
}
type PlanResult struct {
	Draft          PlanDraft `json:"draft"`
	TargetSnapshot Snapshot  `json:"target_snapshot"`
	AppliedEvent   *Event    `json:"applied_event"`
}
type PlanCommit struct {
	RequestID             string     `json:"request_id"`
	ExpectedRevision      int64      `json:"expected_revision"`
	ExpectedCurrentPlanID *string    `json:"expected_current_plan_id"`
	Result                PlanResult `json:"result"`
}
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
type Run struct {
	ID         string         `json:"id"`
	ScenarioID string         `json:"scenario_id"`
	Status     string         `json:"status"`
	PlanID     *string        `json:"plan_id"`
	Error      *ContractError `json:"error"`
}
type DataStore interface {
	GetSnapshot(context.Context, string, int64) (Snapshot, error)
	GetPlan(context.Context, string) (Plan, error)
	CommitPlan(context.Context, PlanCommit) (Plan, error)
}
type PlanService interface {
	Build(context.Context, BuildPlanRequest) (PlanResult, error)
	Replan(context.Context, ReplanRequest) (PlanResult, error)
}
type GeocodeItem = GeocodeResultItem
type Geocoder interface {
	Geocode(context.Context, GeocodeRequest) (GeocodeResult, error)
}

func NewError(code, message string) *ContractError {
	return &ContractError{Code: code, Message: message, Details: map[string]any{}}
}
