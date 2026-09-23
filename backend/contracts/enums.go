package contracts

import shared "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"

type Transport = shared.Transport

const (
	TransportCar  Transport = "car"
	TransportWalk Transport = "walk"
)

type Priority = shared.Priority

const (
	PriorityNormal Priority = "normal"
	PriorityUrgent Priority = "urgent"
)

type OrderStatus = shared.OrderStatus

const (
	OrderStatusActive    OrderStatus = "active"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type SolveMode = shared.SolveMode

const (
	SolveModeBaseline  SolveMode = "baseline"
	SolveModeOptimized SolveMode = "optimized"
)

type Termination = shared.Termination

const (
	TerminationCompleted Termination = "completed"
	TerminationTimeLimit Termination = "time_limit"
)

type EventType = string

const (
	EventUrgentOrderAdded    EventType = "urgent_order_added"
	EventOrderCancelled      EventType = "order_cancelled"
	EventEngineerUnavailable EventType = "engineer_unavailable"
)

type PlanChangeReason = string

const (
	PlanChangeReassigned  PlanChangeReason = "reassigned"
	PlanChangeRescheduled PlanChangeReason = "rescheduled"
	PlanChangeAssigned    PlanChangeReason = "assigned"
	PlanChangeUnassigned  PlanChangeReason = "unassigned"
	PlanChangeCancelled   PlanChangeReason = "cancelled"
)

type RunStatus = string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)

const (
	UnassignedNoMatchingSkill     = "NO_MATCHING_SKILL"
	UnassignedNoMatchingTransport = "NO_MATCHING_TRANSPORT"
	UnassignedNoAvailableEngineer = "NO_AVAILABLE_ENGINEER"
	UnassignedNoReachableRoute    = "NO_REACHABLE_ROUTE"
	UnassignedNoFeasibleSlot      = "NO_FEASIBLE_SLOT"
	UnassignedBySolver            = "NOT_ASSIGNED_BY_SOLVER"
)

const (
	OrderStatusSent         = shared.OrderStatusSent
	OrderStatusEnRoute      = shared.OrderStatusEnRoute
	OrderStatusInProgress   = shared.OrderStatusInProgress
	OrderStatusCompleted    = shared.OrderStatusCompleted
	EventOrderStatusChanged = "order_status_changed"
	PlanChangeStatusChanged = "status_changed"
	WorkTypeEmergency       = shared.WorkTypeEmergency
	WorkTypeConnection      = shared.WorkTypeConnection
	WorkTypeRepair          = shared.WorkTypeRepair
	WorkTypeAdditional      = shared.WorkTypeAdditional
	EquipmentRouter         = shared.EquipmentRouter
	EquipmentTVBox          = shared.EquipmentTVBox
)

type Equipment = shared.Equipment
type OrderExecution = shared.OrderExecution
