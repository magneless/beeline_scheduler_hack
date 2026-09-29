import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';

import { getScenarios } from 'shared/api';

import { useDispatcherWorkspaceStore } from './store';

export const useRegionSwitcher = (currentScenarioId?: string) => {
    const navigate = useNavigate();
    const resetSelection = useDispatcherWorkspaceStore(
        (state) => state.resetSelection
    );
    const scenariosQuery = useQuery({
        queryKey: ['scenarios', 'list'],
        queryFn: getScenarios,
        enabled: Boolean(currentScenarioId),
    });
    const scenarios = scenariosQuery.data?.items ?? [];

    const switchRegion = (scenarioId: string) => {
        if (!scenarioId || scenarioId === currentScenarioId) {
            return;
        }

        if (scenarios.some((item) => item.scenario_id === scenarioId)) {
            resetSelection();
            navigate(`/s/${encodeURIComponent(scenarioId)}`);
        }
    };

    return {
        scenarios,
        pending: scenariosQuery.isPending,
        error: scenariosQuery.error,
        refresh: () => void scenariosQuery.refetch(),
        switchRegion,
    };
};
