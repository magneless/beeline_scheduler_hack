import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { createEvent, type PlanEventInput } from 'features/applyPlanEvent';
import { apiPost } from 'shared/api';
import {
    type PendingChanges,
    type Plan,
    type Snapshot,
} from 'shared/api/types/contracts';
import { commandErrorMessage } from 'shared/lib/commandErrors';

export const usePendingChanges = ({
    scenarioId,
    queue,
    snapshot,
    plan,
    selectedOrderId,
    onReload,
}: {
    scenarioId: string;
    queue?: PendingChanges;
    snapshot?: Snapshot;
    plan?: Plan;
    selectedOrderId: string | null;
    onReload: () => Promise<void>;
}) => {
    const client = useQueryClient();
    const mutation = useMutation({
        mutationFn: async (input: PlanEventInput | 'undo') => {
            if (!queue || !snapshot || !plan) {
                throw new Error('Дождитесь загрузки смены');
            }
            return apiPost<PendingChanges>(
                `/scenarios/${scenarioId}/pending-changes`,
                {
                    expected_revision: queue.revision,
                    snapshot_revision: queue.snapshot_revision,
                    base_plan_id: plan.id,
                    ...(input === 'undo'
                        ? { undo_last: true }
                        : {
                              event: createEvent(
                                  input,
                                  snapshot,
                                  plan,
                                  selectedOrderId
                              ),
                          }),
                }
            );
        },
        onSuccess: async (saved, input) => {
            client.setQueryData(['scenarios', scenarioId, 'pending'], saved);
            client.setQueryData(['scenarios', scenarioId, 'proposal'], null);
            await onReload();
            toast.success(
                input === 'undo'
                    ? 'Последнее изменение отменено'
                    : 'Изменение сохранено. Пересчёт запускается кнопкой.'
            );
        },
        onError: (error) => {
            toast.error(commandErrorMessage(error));
            void onReload();
        },
    });
    return {
        save: (input: PlanEventInput) => mutation.mutate(input),
        undo: () => mutation.mutate('undo'),
        saving: mutation.isPending,
    };
};
