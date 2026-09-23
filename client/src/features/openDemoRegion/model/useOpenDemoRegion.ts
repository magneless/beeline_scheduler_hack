import { useNavigate } from 'react-router-dom';
import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';

import { createScenario } from 'shared/api';

type UseOpenDemoRegionParams = {
    errorFallback?: string;
};

export const useOpenDemoRegion = ({
    errorFallback = 'Не удалось открыть район',
}: UseOpenDemoRegionParams = {}) => {
    const navigate = useNavigate();

    const mutation = useMutation({
        mutationFn: createScenario,
        onSuccess: (scenario) => {
            navigate(`/s/${scenario.snapshot.scenario_id}`);
        },
        onError: (error) => {
            toast.error(error instanceof Error ? error.message : errorFallback);
        },
    });

    return {
        open: (datasetId: string) => mutation.mutate(datasetId),
        pending: mutation.isPending,
    };
};
