package contracts

import (
	"context"
	"time"
)

// Planner assigns eligible orders to engineers and builds their routes.
type Planner interface {
	Solve(ctx context.Context, input SolveRequest) (SolveResult, error)
}

type SolveRequest struct {
	Mode                   SolveMode       `json:"mode"`
	EmergencyFirst         bool            `json:"emergency_first,omitempty"`
	Orders                 []Order         `json:"orders"`
	Engineers              []Engineer      `json:"engineers"`
	EngineerStates         []EngineerState `json:"engineer_states"`
	AlreadyUsedEngineerIDs []string        `json:"already_used_engineer_ids"`
	TravelMatrix           TravelMatrix    `json:"travel_matrix"`
	FixedRoutes            []Route         `json:"fixed_routes"`
	ProtectedLegIDs        []string        `json:"protected_leg_ids"`
	TimeLimitMS            int64           `json:"time_limit_ms"`
}

type SolveResult struct {
	Routes      []Route           `json:"routes"`
	Unassigned  []UnassignedOrder `json:"unassigned"`
	Termination Termination       `json:"termination"`
}

type Order struct {
	ID                string              `json:"id"`
	LocationID        string              `json:"location_id"`
	WorkType          WorkType            `json:"work_type"`
	RequiredSkills    []string            `json:"required_skills"`
	RequiredTransport *Transport          `json:"required_transport"`
	Window            Window              `json:"window"`
	ReceivedAt        time.Time           `json:"received_at"`
	ServiceSec        int64               `json:"service_sec"`
	Priority          Priority            `json:"priority"`
	EquipmentRequired map[Equipment]int64 `json:"equipment_required"`
	SourceOrder       int64               `json:"source_order"`
	Status            OrderStatus         `json:"status"`
	Execution         *OrderExecution     `json:"execution"`
}

type OrderExecution struct {
	EngineerID    string     `json:"engineer_id"`
	DepartedAt    *time.Time `json:"departed_at"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	ExpectedEndAt *time.Time `json:"expected_end_at"`
}

type Engineer struct {
	ID             string              `json:"id"`
	Skills         []string            `json:"skills"`
	Transport      Transport           `json:"transport"`
	Shift          Window              `json:"shift"`
	Available      bool                `json:"available"`
	Reserve        bool                `json:"reserve,omitempty"`
	EquipmentStock map[Equipment]int64 `json:"equipment_stock"`
	SourceOrder    int64               `json:"source_order"`
}

type EngineerState struct {
	EngineerID         string              `json:"engineer_id"`
	StartLocationID    string              `json:"start_location_id"`
	AvailableFrom      time.Time           `json:"available_from"`
	EquipmentAvailable map[Equipment]int64 `json:"equipment_available"`
}

type TravelMatrix struct {
	ID           string                       `json:"id"`
	GeoContextID string                       `json:"geo_context_id"`
	LocationIDs  []string                     `json:"location_ids"`
	Profiles     map[Transport][][]TravelCell `json:"profiles"`
}

type TravelCell struct {
	Reachable   bool   `json:"reachable"`
	DurationSec *int64 `json:"duration_sec"`
	DistanceM   *int64 `json:"distance_m"`
}

type Route struct {
	EngineerID      string    `json:"engineer_id"`
	StartLocationID string    `json:"start_location_id"`
	StartAt         time.Time `json:"start_at"`
	Visits          []Visit   `json:"visits"`
	Legs            []Leg     `json:"legs"`
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

type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type UnassignedOrder struct {
	OrderID    string           `json:"order_id"`
	ReasonCode UnassignedReason `json:"reason_code"`
	Message    string           `json:"message"`
}

type Equipment string

const (
	EquipmentRouter Equipment = "router"
	EquipmentTVBox  Equipment = "tv_box"
)

type Transport string

const (
	TransportCar  Transport = "car"
	TransportWalk Transport = "walk"
)

type WorkType string

const (
	WorkTypeEmergency  WorkType = "emergency"
	WorkTypeConnection WorkType = "connection"
	WorkTypeRepair     WorkType = "repair"
	WorkTypeAdditional WorkType = "additional"
)

type Priority string

const (
	PriorityNormal Priority = "normal"
	PriorityUrgent Priority = "urgent"
)

type OrderStatus string

const (
	OrderStatusActive     OrderStatus = "active"
	OrderStatusSent       OrderStatus = "sent"
	OrderStatusEnRoute    OrderStatus = "en_route"
	OrderStatusInProgress OrderStatus = "in_progress"
	OrderStatusCompleted  OrderStatus = "completed"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

type SolveMode string

const (
	SolveModeBaseline   SolveMode = "baseline"
	SolveModeOptimized  SolveMode = "optimized"
	SolveModeInsertOnly SolveMode = "insert_only"
)

type Termination string

const (
	TerminationCompleted Termination = "completed"
	TerminationTimeLimit Termination = "time_limit"
)

type UnassignedReason string

const (
	ReasonNoMatchingSkill     UnassignedReason = "NO_MATCHING_SKILL"
	ReasonNoMatchingTransport UnassignedReason = "NO_MATCHING_TRANSPORT"
	ReasonNoMatchingEquipment UnassignedReason = "NO_MATCHING_EQUIPMENT"
	ReasonNoAvailableEngineer UnassignedReason = "NO_AVAILABLE_ENGINEER"
	ReasonNoReachableRoute    UnassignedReason = "NO_REACHABLE_ROUTE"
	ReasonNoFeasibleSlot      UnassignedReason = "NO_FEASIBLE_SLOT"
	ReasonNoFeasibleInsertion UnassignedReason = "NO_FEASIBLE_INSERTION"
	ReasonNotAssignedBySolver UnassignedReason = "NOT_ASSIGNED_BY_SOLVER"
)

// ContractError is a structured error shared by backend modules.
type ContractError struct {
	Cause   error          `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func (e *ContractError) Error() string { return e.Code + ": " + e.Message }

func (e *ContractError) Unwrap() error { return e.Cause }
