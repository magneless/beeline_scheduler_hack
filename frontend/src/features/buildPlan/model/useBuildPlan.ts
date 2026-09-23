import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { buildPlan } from 'shared/api';
import {
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

type UseBuildPlanParams = {
    solveMode?: SolveMode;
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
    solveMode,
}: UseBuildPlanParams) => {
    const queryClient = useQueryClient();
    const [runStatus, setRunStatus] = useState<TypeOrNull<Run['status']>>(null);
    const [errorMessage, setErrorMessage] = useState<string>();

    useEffect(() => {
        setErrorMessage(undefined);
    }, [scenarioId]);

    const mutation = useMutation({
        mutationFn: async () => {
            setErrorMessage(undefined);
            if (!snapshot) {
                throw new Error('Сценарий ещё не загружен');
            }
            if (snapshot.engineers.length === 0) {
                throw new Error(
                    'Добавьте бригаду через CSV в разделе «Управление бригадами»'
                );
            }

            const run = await runPlanCommand(
                () =>
                    buildPlan({
                        scenarioId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        solveMode,
                        expectedCurrentPlanId: planId ?? null,
                    }),
                setRunStatus
            );

            await onReload();
            setRunStatus(null);
            return run;
        },
        onError: (error) => {
            const message = commandErrorMessage(error);
            setErrorMessage(message);
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

            toast.error(message);
        },
        onSuccess: () => toast.success('План собран'),
    });

    return {
        build: () => mutation.mutate(),
        pending: mutation.isPending,
        runStatus,
        errorMessage,
    };
};
