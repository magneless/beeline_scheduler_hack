import { useQuery } from '@tanstack/react-query';

import { useOpenDemoRegion } from 'features/openDemoRegion';
import { getDemoDatasets } from 'shared/api';
import { env } from 'shared/config/env';
import { regionLabel } from 'shared/lib/config';

import { useDispatcherWorkspaceStore } from './store';
import { workspaceCopy } from '../lib/config';

const fallbackDatasets = Object.entries(regionLabel).map(([id, name]) => ({
    id,
    name,
    region_id: id,
    date: '',
    timezone: 'Europe/Moscow',
}));

export const useRegionSwitcher = (currentRegionId?: string) => {
    const resetSelection = useDispatcherWorkspaceStore(
        (state) => state.resetSelection
    );
    const datasetsQuery = useQuery({
        queryKey: ['demo-datasets'],
        queryFn: getDemoDatasets,
        initialData:
            env.apiMode === 'mock' ? { items: fallbackDatasets } : undefined,
    });
    const openRegion = useOpenDemoRegion({
        errorFallback: workspaceCopy.regionSwitchError,
    });

    const datasets = datasetsQuery.data?.items ?? [];

    const switchRegion = (regionId: string) => {
        if (!regionId || regionId === currentRegionId) {
            return;
        }

        resetSelection();
        const dataset = datasets.find((item) => item.region_id === regionId);
        if (dataset) {
            openRegion.open(dataset.id);
        }
    };

    return {
        datasets,
        pending: openRegion.pending,
        switchRegion,
    };
};
