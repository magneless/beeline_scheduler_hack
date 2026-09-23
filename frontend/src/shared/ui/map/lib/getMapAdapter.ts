import { env } from 'shared/config/env';

import { type MapAdapter } from './mapContract';
import { createOsmMapAdapter } from './osmMapAdapter';
import { createYandexMapAdapter } from './yandexMapAdapter';

export const getMapAdapter = (): MapAdapter => {
    if (!env.yandexMapsKey) {
        return createOsmMapAdapter();
    }

    return createYandexMapAdapter();
};
