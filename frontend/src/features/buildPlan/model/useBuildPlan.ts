import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { buildPlan } from 'shared/api';
import { type Run, type Snapshot } from 'shared/api/types/contracts';
import {
    commandErrorMessage,
    isStaleVersionError,
} from 'shared/lib/commandErrors';
import { runPlanCommand } from 'shared/lib/planCommand';
import { type TypeOrNull } from 'shared/lib/types';

type UseBuildPlanParams = {
    scenarioId: string;
    snapshot: Snapshot | undefined;
    planId: TypeOrNull<string> | undefined;
    onReload: () => Promise<void>;
};

export const useBuildPlan = ({
    scenarioId,
    snapshot,
    planId,
    onReload,
}: UseBuildPlanParams) => {
    const queryClient = useQueryClient();
    const [runStatus, setRunStatus] = useState<TypeOrNull<Run['status']>>(null);

    const mutation = useMutation({
        mutationFn: async () => {
            if (!snapshot) {
                throw new Error('Сценарий ещё не загружен');
            }
            if (snapshot.engineers.length === 0) {
                throw new Error(
                    'Добавьте хотя бы одну бригаду через CSV в панели «Бригады»'
                );
            }

            const run = await runPlanCommand(
                () =>
                    buildPlan({
                        scenarioId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        expectedCurrentPlanId: planId ?? null,
                    }),
                setRunStatus
            );

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
        onSuccess: () => toast.success('План собран'),
    });

    return {
        build: () => mutation.mutate(),
        pending: mutation.isPending,
        runStatus,
    };
};
