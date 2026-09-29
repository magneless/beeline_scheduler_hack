import type { Plan, PlanEvent, Snapshot } from 'shared/api/types/contracts';

import type { PlanEventInput } from './types';

const engineerForOrder = (plan: Plan, orderId: string) =>
    plan.routes.find((route) =>
        route.visits.some((visit) => visit.order_id === orderId)
    )?.engineer_id;

export const createEvent = (
    input: PlanEventInput,
    snapshot: Snapshot,
    plan: Plan,
    selectedOrderId: string | null
): PlanEvent => {
    const base = { id: crypto.randomUUID(), occurred_at: input.occurredAt };
    if (input.kind === 'new_order') {
        const restored = input.restoreOrderId
            ? snapshot.unlocated_orders?.find(
                  (item) => item.order.id === input.restoreOrderId
              )?.order
            : undefined;
        if (input.restoreOrderId && !restored) {
            throw new Error('Заявка уже обновлена. Обновите сценарий.');
        }
        const locationId =
            restored?.location_id ??
            input.locationId ??
            `location-${crypto.randomUUID()}`;
        const location = input.locationId
            ? undefined
            : {
                  id: locationId,
                  address: input.address?.trim() ?? '',
                  ...(input.point ? { point: input.point } : {}),
              };
        if (!input.locationId && !input.address?.trim()) {
            throw new Error('Укажите адрес');
        }
        return {
            ...base,
            type:
                input.orderType === 'urgent'
                    ? 'urgent_order_added'
                    : 'ordinary_order_added',
            payload: {
                ...(location ? { location } : {}),
                order: {
                    id: restored?.id ?? `order-${crypto.randomUUID()}`,
                    location_id: locationId,
                    work_type: input.workType,
                    required_skills: input.requiredSkills,
                    required_transport: input.transport,
                    window: { start: input.windowStart, end: input.windowEnd },
                    received_at: input.occurredAt,
                    service_sec: input.serviceSec,
                    priority:
                        input.orderType === 'urgent' ? 'urgent' : 'normal',
                    equipment_required: input.equipment,
                    source_order:
                        restored?.source_order ?? snapshot.orders.length + 1,
                    status: 'active',
                    execution: null,
                },
            },
        };
    }
    if (input.kind === 'engineer_unavailable') {
        return {
            ...base,
            type: 'engineer_unavailable',
            payload: { engineer_id: input.engineerId },
        };
    }
    if (input.kind === 'cancel') {
        if (!selectedOrderId) {
            throw new Error('Выберите заявку');
        }
        return {
            ...base,
            type: 'order_cancelled',
            payload: { order_id: selectedOrderId, reason: input.reason },
        };
    }
    if (input.kind === 'cancel_many') {
        if (input.orderIds.length === 0) {
            throw new Error('Выберите заявки');
        }
        if (
            new Set(input.orderIds).size !== input.orderIds.length ||
            input.orderIds.some(
                (id) =>
                    !snapshot.orders.some(
                        (order) =>
                            order.id === id &&
                            order.status !== 'completed' &&
                            order.status !== 'cancelled'
                    ) ||
                    plan.routes.some((route) =>
                        route.visits.some((visit) => visit.order_id === id)
                    )
            )
        ) {
            throw new Error('Список заявок изменился. Обновите смену.');
        }
        return {
            ...base,
            type: 'order_cancelled',
            payload: { order_ids: input.orderIds, reason: input.reason },
        };
    }
    const order = snapshot.orders.find((item) => item.id === selectedOrderId);
    const engineerId =
        order?.execution?.engineer_id ??
        (selectedOrderId ? engineerForOrder(plan, selectedOrderId) : undefined);
    if (!selectedOrderId || !engineerId) {
        throw new Error('У заявки нет исполнителя');
    }
    return {
        ...base,
        type: 'order_status_changed',
        payload: {
            order_id: selectedOrderId,
            status: input.status,
            engineer_id: engineerId,
            expected_end_at:
                input.status === 'in_progress'
                    ? (input.expectedEndAt ?? null)
                    : null,
        },
    };
};
