import { useQuery } from '@tanstack/react-query';

import { useImportOrders } from 'features/importOrders';
import { useOpenDemoRegion } from 'features/openDemoRegion';
import { type DemoDataset, getDemoDatasets } from 'shared/api';

import { demoRegions, setupCopy } from '../lib/config';

export const useScenarioSetup = () => {
    const datasetsQuery = useQuery({
        queryKey: ['demo-datasets'],
        queryFn: getDemoDatasets,
        initialData: { items: [...demoRegions] },
    });

    const openRegion = useOpenDemoRegion({
        errorFallback: setupCopy.openRegionError,
    });
    const importOrders = useImportOrders({
        errorFallback: setupCopy.importError,
        onSkippedRows: setupCopy.skippedRows,
    });

    const datasets: DemoDataset[] = datasetsQuery.data.items.length
        ? datasetsQuery.data.items
        : [...demoRegions];

    return {
        datasets,
        pending: openRegion.pending || importOrders.pending,
        isImporting: importOrders.pending,
        openRegion: openRegion.open,
        importOrders: importOrders.importFile,
    };
};
