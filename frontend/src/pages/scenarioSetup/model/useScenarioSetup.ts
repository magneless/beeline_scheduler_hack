import { useQuery } from '@tanstack/react-query';

import { useImportOrders } from 'features/importOrders';
import { useOpenDemoRegion } from 'features/openDemoRegion';
import { type DemoDataset, getDemoDatasets } from 'shared/api';
import { env } from 'shared/config/env';

import { demoRegions, setupCopy } from '../lib/config';

export const useScenarioSetup = () => {
    const datasetsQuery = useQuery({
        queryKey: ['demo-datasets'],
        queryFn: getDemoDatasets,
        initialData:
            env.apiMode === 'mock' ? { items: [...demoRegions] } : undefined,
    });

    const openRegion = useOpenDemoRegion({
        errorFallback: setupCopy.openRegionError,
    });
    const importOrders = useImportOrders({
        errorFallback: setupCopy.importError,
        onSkippedRows: setupCopy.skippedRows,
    });

    const datasets: DemoDataset[] = datasetsQuery.data?.items ?? [];

    return {
        datasets,
        pending: openRegion.pending || importOrders.pending,
        openRegionError: openRegion.error,
        retryOpenRegion: openRegion.retry,
        isImporting: importOrders.pending,
        openRegion: openRegion.open,
        importOrders: importOrders.importFile,
    };
};
