import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { importScenario } from 'shared/api';
import { useRequestProgress } from 'shared/lib/useRequestProgress';
import { formatCount } from 'shared/lib/utils';

type UseImportOrdersParams = {
    errorFallback?: string;
    onSkippedRows?: (countLabel: string) => string;
};

export const useImportOrders = ({
    errorFallback = 'Не удалось загрузить CSV',
    onSkippedRows = (countLabel) => `${countLabel} не вошла в расчёт`,
}: UseImportOrdersParams = {}) => {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const progress = useRequestProgress();

    const mutation = useMutation({
        mutationFn: (input: Parameters<typeof importScenario>[0]) =>
            importScenario(input, progress.callbacks),
        onMutate: progress.start,
        onSettled: progress.finish,
        onSuccess: (scenario) => {
            const scenarioId = scenario.snapshot.scenario_id;
            queryClient.invalidateQueries({ queryKey: ['scenarios', 'list'] });
            const unlocated = new Set(
                scenario.snapshot.unlocated_orders?.map(
                    (item) => item.order.location_id
                )
            );
            const issueCount = new Set(
                scenario.snapshot.issues
                    .filter(
                        (issue) =>
                            issue.source_row &&
                            !unlocated.has(issue.entity_id ?? '')
                    )
                    .map((issue) => issue.source_row)
            ).size;

            if (issueCount) {
                toast.message(
                    onSkippedRows(
                        formatCount(issueCount, ['строка', 'строки', 'строк'])
                    )
                );
            }

            queryClient.setQueryData(
                ['scenarios', scenarioId, 'current'],
                scenario
            );
            queryClient.removeQueries({
                queryKey: ['plans', scenarioId],
            });

            navigate(`/s/${scenarioId}`);
        },
        onError: (error) => {
            toast.error(error instanceof Error ? error.message : errorFallback);
        },
    });

    return {
        importFile: (input: Parameters<typeof importScenario>[0]) =>
            mutation.mutate(input),
        error: mutation.error,
        resetError: mutation.reset,
        pending: mutation.isPending,
        progress: progress.state,
    };
};
