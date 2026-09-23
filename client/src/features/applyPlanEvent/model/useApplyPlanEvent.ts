import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { postPlanEvent } from 'shared/api';
import { type Plan, type Run, type Snapshot } from 'shared/api/types/contracts';
import {
    commandErrorMessage,
    isStaleVersionError,
} from 'shared/lib/commandErrors';
import { runPlanCommand } from 'shared/lib/planCommand';
import { type TypeOrNull } from 'shared/lib/types';

import { type PlanEventInput } from './types';

type UseApplyPlanEventParams = {
    scenarioId: string;
    planId: TypeOrNull<string> | undefined;
    snapshot: Snapshot | undefined;
    plan: Plan | undefined;
    selectedOrderId: TypeOrNull<string>;
    onReload: () => Promise<void>;
};

const engineerForOrder = (plan: Plan, orderId: string) =>
    plan.routes
        .flatMap((route) =>
            route.visits.map((visit) => ({
                orderId: visit.order_id,
                engineerId: route.engineer_id,
            }))
        )
        .find((item) => item.orderId === orderId)?.engineerId;

export const useApplyPlanEvent = ({
    scenarioId,
    planId,
    snapshot,
    plan,
    selectedOrderId,
    onReload,
}: UseApplyPlanEventParams) => {
    const queryClient = useQueryClient();
    const [runStatus, setRunStatus] = useState<TypeOrNull<Run['status']>>(null);

    const mutation = useMutation({
        mutationFn: async (input: PlanEventInput) => {
            if (!snapshot || !plan || !planId) {
                throw new Error('Сначала соберите план');
            }

            const order = selectedOrderId
                ? snapshot.orders.find((item) => item.id === selectedOrderId)
                : undefined;
            const engineerId =
                order?.execution?.engineer_id ??
                (selectedOrderId
                    ? engineerForOrder(plan, selectedOrderId)
                    : undefined);

            const run = await runPlanCommand(async () => {
                if (input.kind === 'urgent') {
                    return postPlanEvent({
                        planId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        event: {
                            id: crypto.randomUUID(),
                            occurred_at: input.occurredAt,
                            type: 'urgent_order_added',
                            payload: {
                                order: {
                                    id: 'order-10',
                                    location_id: 'loc-10',
                                    work_type: 'emergency',
                                    required_skills: ['emergency'],
                                    required_transport: 'car',
                                    window: {
                                        start: `${snapshot.date}T06:00:00Z`,
                                        end: `${snapshot.date}T15:00:00Z`,
                                    },
                                    received_at: input.occurredAt,
                                    service_sec: 4800,
                                    priority: 'urgent',
                                    equipment_required: { router: 1 },
                                    source_order: snapshot.orders.length + 1,
                                    status: 'active',
                                    execution: null,
                                },
                            },
                        },
                    });
                }

                if (input.kind === 'engineer_unavailable') {
                    return postPlanEvent({
                        planId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        event: {
                            id: crypto.randomUUID(),
                            occurred_at: input.occurredAt,
                            type: 'engineer_unavailable',
                            payload: { engineer_id: input.engineerId },
                        },
                    });
                }

                if (input.kind === 'cancel') {
                    if (!selectedOrderId) {
                        throw new Error('Выберите заявку');
                    }

                    return postPlanEvent({
                        planId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        event: {
                            id: crypto.randomUUID(),
                            occurred_at: input.occurredAt,
                            type: 'order_cancelled',
                            payload: {
                                order_id: selectedOrderId,
                                reason: input.reason,
                            },
                        },
                    });
                }

                if (!selectedOrderId || !engineerId) {
                    throw new Error('У заявки нет исполнителя');
                }

                return postPlanEvent({
                    planId,
                    requestId: crypto.randomUUID(),
                    snapshotRevision: snapshot.revision,
                    event: {
                        id: crypto.randomUUID(),
                        occurred_at: input.occurredAt,
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
                    },
                });
            }, setRunStatus);

            await onReload();
            setRunStatus(null);
            return run;
        },
        onError: (error) => {
            if (isStaleVersionError(error)) {
                toast.error('Данные устарели. Обновите смену.', {
                    action: {
                        label: 'Обновить',
                        onClick: () => {
                            void queryClient.invalidateQueries({
                                queryKey: ['scenarios', scenarioId],
                            });
                        },
                    },
                });
                return;
            }

            toast.error(commandErrorMessage(error));
        },
        onSuccess: () => toast.success('План пересчитан'),
    });

    return {
        apply: (input: PlanEventInput) => mutation.mutate(input),
        pending: mutation.isPending,
        runStatus,
    };
};
