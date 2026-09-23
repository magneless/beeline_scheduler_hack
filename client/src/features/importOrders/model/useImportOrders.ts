import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { importScenario } from 'shared/api';
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

    const mutation = useMutation({
        mutationFn: importScenario,
        onSuccess: (scenario) => {
            const scenarioId = scenario.snapshot.scenario_id;
            const issueCount = scenario.snapshot.issues.length;

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
        importFile: (input: { file: File; regionId: string; date: string }) =>
            mutation.mutate(input),
        pending: mutation.isPending,
    };
};
