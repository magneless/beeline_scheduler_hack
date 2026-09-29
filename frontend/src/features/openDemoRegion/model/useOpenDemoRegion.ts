import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { createScenario } from 'shared/api';
import { useRequestProgress } from 'shared/lib/useRequestProgress';

type UseOpenDemoRegionParams = {
    errorFallback?: string;
};

export const useOpenDemoRegion = ({
    errorFallback = 'Не удалось открыть район',
}: UseOpenDemoRegionParams = {}) => {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [lastDatasetId, setLastDatasetId] = useState<string>();
    const progress = useRequestProgress();

    const mutation = useMutation({
        mutationFn: (datasetId: string) =>
            createScenario(datasetId, progress.callbacks),
        onMutate: progress.start,
        onSettled: progress.finish,
        onSuccess: (scenario) => {
            const scenarioId = scenario.snapshot.scenario_id;

            queryClient.invalidateQueries({ queryKey: ['scenarios', 'list'] });

            queryClient.setQueryData(
                ['scenarios', scenarioId, 'current'],
                scenario
            );
            queryClient.removeQueries({
                queryKey: ['plans', scenarioId],
            });
            queryClient.removeQueries({
                queryKey: ['scenarios', scenarioId],
                predicate: (query) => query.queryKey[2] !== 'current',
            });

            navigate(`/s/${scenarioId}`);
        },
        onError: (error) => {
            toast.error(error instanceof Error ? error.message : errorFallback);
        },
    });

    return {
        open: (datasetId: string) => {
            setLastDatasetId(datasetId);
            mutation.mutate(datasetId);
        },
        retry: () => {
            if (lastDatasetId) {
                mutation.mutate(lastDatasetId);
            }
        },
        pending: mutation.isPending,
        progress: progress.state,
        error: mutation.error,
    };
};
