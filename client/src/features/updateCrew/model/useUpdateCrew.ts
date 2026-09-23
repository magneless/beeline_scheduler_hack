import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { patchEngineer } from 'shared/api';
import { type Snapshot } from 'shared/api/types/contracts';
import {
    commandErrorMessage,
    isStaleVersionError,
} from 'shared/lib/commandErrors';

import { type CrewPatchInput } from './types';

type UseUpdateCrewParams = {
    scenarioId: string;
    snapshot: Snapshot | undefined;
    onReload: () => Promise<void>;
};

export const useUpdateCrew = ({
    scenarioId,
    snapshot,
    onReload,
}: UseUpdateCrewParams) => {
    const queryClient = useQueryClient();

    const mutation = useMutation({
        mutationFn: async ({
            engineerId,
            ...fields
        }: CrewPatchInput & { engineerId: string }) => {
            if (!snapshot) {
                throw new Error('Сценарий ещё не загружен');
            }

            return patchEngineer({
                scenarioId,
                engineerId,
                expectedRevision: snapshot.revision,
                ...fields,
            });
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
        onSuccess: async () => {
            await onReload();
            toast.success('Бригада обновлена');
        },
    });

    return {
        update: (engineerId: string, patch: CrewPatchInput) =>
            mutation.mutate({ engineerId, ...patch }),
        pending: mutation.isPending,
    };
};
