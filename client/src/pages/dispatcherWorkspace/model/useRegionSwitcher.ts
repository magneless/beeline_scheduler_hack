import { useQuery } from '@tanstack/react-query';

import { useOpenDemoRegion } from 'features/openDemoRegion';
import { getDemoDatasets } from 'shared/api';
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
        initialData: { items: fallbackDatasets },
    });
    const openRegion = useOpenDemoRegion({
        errorFallback: workspaceCopy.regionSwitchError,
    });

    const datasets =
        datasetsQuery.data.items.length > 0
            ? datasetsQuery.data.items
            : fallbackDatasets;

    const switchRegion = (regionId: string) => {
        if (!regionId || regionId === currentRegionId) {
            return;
        }

        resetSelection();
        openRegion.open(regionId);
    };

    return {
        datasets,
        pending: openRegion.pending,
        switchRegion,
    };
};
