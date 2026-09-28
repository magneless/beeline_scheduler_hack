import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { postPlanEvent } from 'shared/api';
import {
    type Plan,
    type Run,
    type Snapshot,
    type SolveMode,
} from 'shared/api/types/contracts';
import {
    commandErrorMessage,
    isStaleVersionError,
} from 'shared/lib/commandErrors';
import { runPlanCommand } from 'shared/lib/planCommand';
import { type TypeOrNull } from 'shared/lib/types';
import { type CalculationState } from 'shared/ui/calculationProgress';

import { createEvent } from './createEvent';
import { isUnassignedCancellation } from './eventRouting';

import { type PlanEventInput } from './types';

type UseApplyPlanEventParams = {
    solveMode?: SolveMode;
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
    solveMode,
}: UseApplyPlanEventParams) => {
    const queryClient = useQueryClient();
    const [runStatus, setRunStatus] = useState<TypeOrNull<Run['status']>>(null);
    const [calculation, setCalculation] = useState<CalculationState | null>(
        null
    );

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

            const startedAt = Date.now();
            const plannedEnd = plan.routes
                .flatMap((route) => route.visits)
                .find((visit) => visit.order_id === selectedOrderId)?.end_at;
            const keepsSchedule =
                input.kind === 'status' &&
                (input.status === 'in_progress' ||
                    (input.status === 'completed' &&
                        plannedEnd &&
                        Date.parse(input.occurredAt) <=
                            Date.parse(plannedEnd)));
            const cancelsUnassigned = isUnassignedCancellation(
                input,
                plan,
                selectedOrderId
            );
            setCalculation({
                startedAt,
                lastEventAt: startedAt,
                ...(keepsSchedule || cancelsUnassigned
                    ? {
                          title: cancelsUnassigned
                              ? input.kind === 'cancel_many'
                                  ? 'Отмена заявок'
                                  : 'Отмена заявки'
                              : 'Сохранение работы',
                          progress: {
                              stage: 'saving',
                              message: cancelsUnassigned
                                  ? 'Сохраняем отмену заявки'
                                  : 'Сохраняем состояние работы',
                              completed: 0,
                              total: 0,
                          },
                      }
                    : {}),
            });
            const run = await runPlanCommand(
                async () => {
                    if (input.kind === 'new_order') {
                        return postPlanEvent({
                            planId,
                            requestId: crypto.randomUUID(),
                            snapshotRevision: snapshot.revision,
                            solveMode,
                            event: createEvent(
                                input,
                                snapshot,
                                plan,
                                selectedOrderId
                            ),
                        });
                    }

                    if (input.kind === 'engineer_unavailable') {
                        return postPlanEvent({
                            planId,
                            requestId: crypto.randomUUID(),
                            snapshotRevision: snapshot.revision,
                            solveMode,
                            event: {
                                id: crypto.randomUUID(),
                                occurred_at: input.occurredAt,
                                type: 'engineer_unavailable',
                                payload: { engineer_id: input.engineerId },
                            },
                        });
                    }

                    if (
                        input.kind === 'cancel' ||
                        input.kind === 'cancel_many'
                    ) {
                        return postPlanEvent({
                            planId,
                            requestId: crypto.randomUUID(),
                            snapshotRevision: snapshot.revision,
                            solveMode,
                            event: createEvent(
                                input,
                                snapshot,
                                plan,
                                selectedOrderId
                            ),
                        });
                    }

                    if (!selectedOrderId || !engineerId) {
                        throw new Error('У заявки нет исполнителя');
                    }

                    return postPlanEvent({
                        planId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        solveMode,
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
                },
                (status) => {
                    setRunStatus(status);
                    setCalculation(
                        (current) =>
                            current && {
                                ...current,
                                lastEventAt: Date.now(),
                            }
                    );
                }
            );

            await onReload();
            setRunStatus(null);
            setCalculation(null);
            return run;
        },
        onError: (error) => {
            setCalculation(null);
            setRunStatus(null);
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
        onSuccess: (_run, input) =>
            toast.success(
                input.kind === 'status'
                    ? input.status === 'completed'
                        ? 'Работа завершена'
                        : 'Состояние работы сохранено'
                    : input.kind === 'cancel_many'
                      ? `Отменено заявок: ${input.orderIds.length}`
                      : input.kind === 'cancel'
                        ? 'Заявка отменена'
                        : 'План пересчитан'
            ),
    });

    return {
        apply: (input: PlanEventInput) => mutation.mutate(input),
        pending: mutation.isPending,
        runStatus,
        calculation,
    };
};
