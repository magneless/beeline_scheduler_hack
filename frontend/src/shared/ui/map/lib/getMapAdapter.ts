import { env } from 'shared/config/env';

import { type MapAdapter } from './mapContract';
import { createPreviewMapAdapter } from './previewMapAdapter';
import { createYandexMapAdapter } from './yandexMapAdapter';

export const getMapAdapter = (): MapAdapter => {
    if (!env.yandexMapsKey) {
        return createPreviewMapAdapter();
    }

    return createYandexMapAdapter();
};
