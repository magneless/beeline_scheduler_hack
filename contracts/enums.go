package contracts

type Transport string

const (
	TransportCar  Transport = "car"
	TransportWalk Transport = "walk"
)

type Priority string

const (
	PriorityNormal Priority = "normal"
	PriorityUrgent Priority = "urgent"
)

type OrderStatus string

const (
	OrderStatusActive    OrderStatus = "active"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type SolveMode string

const (
	SolveModeBaseline  SolveMode = "baseline"
	SolveModeOptimized SolveMode = "optimized"
)

type Termination string

const (
	TerminationCompleted Termination = "completed"
	TerminationTimeLimit Termination = "time_limit"
)

type EventType string

const (
	EventUrgentOrderAdded    EventType = "urgent_order_added"
	EventOrderCancelled      EventType = "order_cancelled"
	EventEngineerUnavailable EventType = "engineer_unavailable"
)

type PlanChangeReason string

const (
	PlanChangeReassigned  PlanChangeReason = "reassigned"
	PlanChangeRescheduled PlanChangeReason = "rescheduled"
	PlanChangeAssigned    PlanChangeReason = "assigned"
	PlanChangeUnassigned  PlanChangeReason = "unassigned"
	PlanChangeCancelled   PlanChangeReason = "cancelled"
)

type RunStatus string

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
