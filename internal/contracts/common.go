package contracts

import "time"

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
	OrderID string           `json:"order_id"`
	Before  *Assignment      `json:"before"`
	After   *Assignment      `json:"after"`
	Reason  PlanChangeReason `json:"reason"`
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
	Order         *Order             `json:"order,omitempty"`
	Location      *LocationInput     `json:"location,omitempty"`
	OrderID       string             `json:"order_id,omitempty"`
	EngineerID    string             `json:"engineer_id,omitempty"`
	Reason        CancellationReason `json:"reason,omitempty"`
	Status        OrderStatus        `json:"status,omitempty"`
	ExpectedEndAt *time.Time         `json:"expected_end_at,omitempty"`
}

type EventType string

const (
	EventUrgentOrderAdded    EventType = "urgent_order_added"
	EventOrderCancelled      EventType = "order_cancelled"
	EventEngineerUnavailable EventType = "engineer_unavailable"
	EventOrderStatusChanged  EventType = "order_status_changed"
)

type CancellationReason string

const (
	CancellationClientRefusal CancellationReason = "client_refusal"
	CancellationCannotPerform CancellationReason = "cannot_perform"
)

type PlanChangeReason string

const (
	PlanChangeReassigned    PlanChangeReason = "reassigned"
	PlanChangeRescheduled   PlanChangeReason = "rescheduled"
	PlanChangeAssigned      PlanChangeReason = "assigned"
	PlanChangeUnassigned    PlanChangeReason = "unassigned"
	PlanChangeCancelled     PlanChangeReason = "cancelled"
	PlanChangeStatusChanged PlanChangeReason = "status_changed"
)

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)
