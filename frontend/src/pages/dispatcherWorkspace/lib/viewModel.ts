import { DateTime } from 'luxon';

import { type Metrics, type Plan, type Snapshot } from 'shared/api';
import { type TypeOrNull } from 'shared/lib/types';

import { defaultOccurredAt } from './eventTime';
import { assignmentOf, buildSchedule, focusClock } from './utils';

type BuildWorkspaceViewParams = {
    snapshot: Snapshot | undefined;
    plan: Plan | undefined;
    compareMetrics: TypeOrNull<Metrics> | undefined;
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
};

export const buildWorkspaceView = ({
    snapshot,
    plan,
    compareMetrics,
    selectedOrderId,
    selectedEngineerId,
}: BuildWorkspaceViewParams) => {
    const activePlan =
        plan && snapshot && plan.scenario_id === snapshot.scenario_id
            ? plan
            : undefined;
    const timezone = snapshot?.timezone ?? 'Europe/Moscow';
    const distances = Object.fromEntries(
        activePlan?.metrics.per_engineer.map((item) => [
            item.engineer_id,
            item.distance_m,
        ]) ?? []
    );
    const baselineDistances = Object.fromEntries(
        compareMetrics?.per_engineer.map((item) => [
            item.engineer_id,
            item.distance_m,
        ]) ?? []
    );
    const assignedCounts = Object.fromEntries(
        activePlan?.routes.map((route) => [
            route.engineer_id,
            route.visits.length,
        ]) ?? []
    );
    const lanes =
        snapshot && activePlan
            ? buildSchedule(snapshot, activePlan, timezone)
            : [];
    const now = DateTime.now().setZone(timezone);
    const focusAt = lanes[0]
        ? focusClock(lanes[0].start, lanes[0].end, now)
        : now;

    const engineerByOrder = new Map<string, string>();
    const visitByOrder = new Map(
        activePlan?.routes.flatMap((route) =>
            route.visits.map((visit) => [visit.order_id, visit] as const)
        ) ?? []
    );

    activePlan?.routes.forEach((route) => {
        route.visits.forEach((visit) => {
            engineerByOrder.set(visit.order_id, route.engineer_id);
        });
    });

    const unassigned = new Map(
        activePlan?.unassigned.map((item) => [item.order_id, item]) ?? []
    );
    const visibleOrders =
        snapshot?.orders.filter((order) => {
            if (selectedEngineerId) {
                return (
                    assignmentOf(order.id, engineerByOrder) ===
                    selectedEngineerId
                );
            }

            return true;
        }) ?? [];
    const issues = [
        ...new Map(
            [...(snapshot?.issues ?? []), ...(activePlan?.issues ?? [])].map(
                (issue) => [
                    `${issue.code}:${issue.entity_id ?? ''}:${issue.source_row ?? ''}:${issue.message}`,
                    issue,
                ]
            )
        ).values(),
    ];
    const canEditEngineers =
        !activePlan?.base_plan_id &&
        !(snapshot?.orders.some((order) => order.execution) ?? false);
    const occurredAtDefault = snapshot
        ? defaultOccurredAt(snapshot.date, timezone, activePlan?.as_of)
        : '';
    const addressByOrder = Object.fromEntries(
        (snapshot?.orders ?? []).map((order) => {
            const address = snapshot?.locations.find(
                (location) => location.id === order.location_id
            )?.address;

            return [order.id, address ?? ''];
        })
    );
    const urgentDone = Boolean(
        snapshot?.orders.some((order) => order.id === 'order-10')
    );

    return {
        timezone,
        distances,
        baselineDistances,
        assignedCounts,
        lanes,
        focusAt,
        engineerByOrder,
        visitByOrder,
        unassigned,
        visibleOrders,
        issues,
        canEditEngineers,
        occurredAtDefault,
        addressByOrder,
        urgentDone,
        selectedOrder: snapshot?.orders.find(
            (order) => order.id === selectedOrderId
        ),
        selectedVisit: selectedOrderId
            ? visitByOrder.get(selectedOrderId)
            : undefined,
        ordersCount: snapshot?.orders.length ?? 0,
        crewsCount: snapshot?.engineers.length ?? 0,
        canEvent: Boolean(activePlan),
    };
};
