import { type TypeOrNull } from 'shared/lib/types';

export type Point = {
    lat: number;
    lon: number;
};

export type TimeWindow = {
    start: string;
    end: string;
};

export type Transport = 'car' | 'walk';
export type WorkType = 'emergency' | 'connection' | 'repair' | 'additional';
export type Priority = 'normal' | 'urgent';
export type OrderStatus =
    'active' | 'sent' | 'en_route' | 'in_progress' | 'completed' | 'cancelled';
export type Equipment = 'router' | 'tv_box';

export type Location = {
    id: string;
    address: string;
    point: Point;
};

export type OrderExecution = {
    engineer_id: string;
    departed_at: TypeOrNull<string>;
    started_at: TypeOrNull<string>;
    finished_at: TypeOrNull<string>;
    expected_end_at: TypeOrNull<string>;
};

export type Order = {
    id: string;
    location_id: string;
    work_type: WorkType;
    required_skills: string[];
    required_transport: TypeOrNull<Transport>;
    window: TimeWindow;
    received_at: string;
    service_sec: number;
    priority: Priority;
    equipment_required: Partial<Record<Equipment, number>>;
    source_order: number;
    status: OrderStatus;
    execution: TypeOrNull<OrderExecution>;
};

export type Engineer = {
    id: string;
    skills: string[];
    transport: Transport;
    shift: TimeWindow;
    available: boolean;
    reserve?: boolean;
    equipment_stock: Partial<Record<Equipment, number>>;
    source_order: number;
};

export type Issue = {
    source_row: TypeOrNull<number>;
    entity_id: TypeOrNull<string>;
    field: TypeOrNull<string>;
    code: string;
    message: string;
};

export type Snapshot = {
    scenario_id: string;
    revision: number;
    region_id: string;
    date: string;
    timezone: string;
    office_location_id: string;
    locations: Location[];
    orders: Order[];
    unlocated_orders?: {
        order: Order;
        address: string;
        message: string;
        candidates?: { address: string; point: Point; source?: string }[];
    }[];
    engineers: Engineer[];
    issues: Issue[];
};

export type ScenarioView = {
    snapshot: Snapshot;
    current_plan_id: TypeOrNull<string>;
};

export type Visit = {
    order_id: string;
    arrival_at: string;
    start_at: string;
    end_at: string;
};

export type Leg = {
    id: string;
    from_location_id: string;
    to_location_id: string;
    start_at: string;
    end_at: string;
    distance_m: number;
    geo_context_id: string;
    geometry: Point[];
};

export type Route = {
    engineer_id: string;
    start_location_id: string;
    start_at: string;
    visits: Visit[];
    legs: Leg[];
};

export type UnassignedOrder = {
    order_id: string;
    reason_code: string;
    message: string;
};

export type Metrics = {
    assigned_count: number;
    completed_count: number;
    unassigned_count: number;
    used_engineer_count: number;
    total_distance_m: number;
    per_engineer: Array<{ engineer_id: string; distance_m: number }>;
};

export type Assignment = {
    engineer_id: string;
    sequence: number;
    arrival_at: string;
    start_at: string;
    end_at: string;
};

export type PlanChange = {
    order_id: string;
    before: TypeOrNull<Assignment>;
    after: TypeOrNull<Assignment>;
    reason:
        | 'reassigned'
        | 'rescheduled'
        | 'assigned'
        | 'unassigned'
        | 'cancelled'
        | 'status_changed';
};

export type SolveMode = 'baseline' | 'optimized';

export type Plan = {
    solve_mode?: SolveMode;
    id: string;
    scenario_id: string;
    snapshot_revision: number;
    base_plan_id: TypeOrNull<string>;
    as_of: string;
    routes: Route[];
    unassigned: UnassignedOrder[];
    cancelled_order_ids: string[];
    completed_order_ids: string[];
    issues: Issue[];
    metrics: Metrics;
    baseline_metrics: TypeOrNull<Metrics>;
    changes: PlanChange[];
    termination: 'completed' | 'time_limit';
    equipment_remaining: Record<string, Partial<Record<Equipment, number>>>;
    option_key?: string;
    lateness?: OrderLateness[];
    reserve_engineer_ids?: string[];
    deferred_order_ids?: string[];
};

export type PlanDraft = Omit<Plan, 'id'>;

export type OrderLateness = {
    order_id: string;
    window: TimeWindow;
    arrival_at: string;
    start_at: string;
    late_sec: number;
};

export type PlanOption = {
    key: 'strict' | 'late_emergency' | 'reserve' | 'original';
    label: string;
    result: {
        draft: PlanDraft;
        target_snapshot: Snapshot;
        applied_event: PlanEvent | null;
    };
    lateness: OrderLateness[];
    reserve_engineer_ids: string[];
    identical_to?: string;
};

export type PlanProposal = {
    id: string;
    scenario_id: string;
    snapshot_revision: number;
    expected_current_plan_id: TypeOrNull<string>;
    event?: PlanEvent | null;
    options: PlanOption[];
};

export type Run = {
    id: string;
    scenario_id: string;
    status: 'queued' | 'running' | 'succeeded' | 'failed';
    plan_id: TypeOrNull<string>;
    error: TypeOrNull<ApiError>;
};

export type DemoDataset = {
    id: string;
    name: string;
    region_id: string;
    date: string;
    timezone: string;
};

export type ApiError = {
    code: string;
    message: string;
    details: Record<string, unknown>;
};

export type EventType =
    | 'urgent_order_added'
    | 'ordinary_order_added'
    | 'order_cancelled'
    | 'engineer_unavailable'
    | 'order_status_changed';

export type CancelReason = 'client_refusal' | 'cannot_perform';

export type PlanEvent = {
    id: string;
    occurred_at: string;
    type: EventType;
    payload: Record<string, unknown>;
};
