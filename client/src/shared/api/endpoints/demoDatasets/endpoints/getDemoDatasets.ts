import { apiGet } from 'shared/api/instance/httpClient';
import { type DemoDataset } from 'shared/api/types/contracts';

import { getDemoDatasetsUrl } from '../../getUrl';

export const getDemoDatasets = () =>
    apiGet<{ items: DemoDataset[] }>(getDemoDatasetsUrl());
