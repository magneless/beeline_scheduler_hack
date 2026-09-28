import {
    AttributionControl,
    Map as VectorMap,
    NavigationControl,
    ScaleControl,
    setWorkerUrl,
    type StyleSpecification,
} from 'maplibre-gl';
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';

import { type MapPoint } from './mapContract';

import 'maplibre-gl/dist/maplibre-gl.css';
import styles from '../ui/VectorMap.module.scss';

setWorkerUrl(workerUrl);

const STYLE_URL = 'https://tiles.openfreemap.org/styles/positron';

const localizeStyle = (style: StyleSpecification): StyleSpecification => {
    for (const layer of style.layers) {
        if (
            layer.type === 'symbol' &&
            layer.layout?.['text-field'] &&
            JSON.stringify(layer.layout['text-field']).includes('name')
        ) {
            layer.layout['text-field'] = [
                'coalesce',
                ['get', 'name:ru'],
                ['get', 'name'],
                ['get', 'name:latin'],
            ];
        }
    }
    // Positron omits house labels; dispatchers need them when choosing a door.
    if (style.sources.openmaptiles) {
        style.layers.push({
            id: 'dispatch-house-numbers',
            type: 'symbol',
            source: 'openmaptiles',
            'source-layer': 'housenumber',
            minzoom: 16,
            layout: {
                'text-field': ['get', 'housenumber'],
                'text-font': ['Noto Sans Regular'],
                'text-size': 11,
            },
            paint: {
                'text-color': '#555b57',
                'text-halo-color': '#ffffff',
                'text-halo-width': 1,
            },
        });
    }
    return style;
};

export const createVectorMap = (
    container: HTMLElement,
    center: MapPoint,
    zoom: number
) => {
    container.classList.add(styles.root);
    const status = document.createElement('div');
    status.className = styles.status;
    status.setAttribute('role', 'status');
    const text = document.createElement('span');
    const retry = document.createElement('button');
    retry.type = 'button';
    retry.textContent = 'Повторить';
    status.append(text, retry);
    status.hidden = true;
    let map: VectorMap;
    try {
        map = new VectorMap({
            container,
            center: [center.lon, center.lat],
            zoom,
            maxZoom: 20,
            dragRotate: false,
            pitchWithRotate: false,
            touchPitch: false,
            attributionControl: false,
            locale: {
                'NavigationControl.ZoomIn': 'Приблизить',
                'NavigationControl.ZoomOut': 'Отдалить',
                'AttributionControl.ToggleAttribution': 'Источники карты',
                'Map.Title': 'Интерактивная карта',
            },
            style: {
                version: 8,
                sources: {},
                layers: [
                    {
                        id: 'background',
                        type: 'background',
                        paint: { 'background-color': '#eef1f0' },
                    },
                ],
            },
        });
    } catch {
        text.textContent =
            'Карта недоступна: включите аппаратное ускорение браузера и обновите страницу.';
        retry.hidden = true;
        status.hidden = false;
        container.append(status);
        return {
            map: null,
            destroy: () => {
                status.remove();
                container.classList.remove(styles.root);
            },
        };
    }
    container.append(status);
    map.touchZoomRotate.disableRotation();
    map.addControl(new NavigationControl({ showCompass: false }), 'top-right');
    map.addControl(new ScaleControl({ unit: 'metric' }), 'bottom-left');
    map.addControl(new AttributionControl({ compact: true }), 'bottom-right');
    let destroyed = false;
    let request: AbortController | null = null;
    const showError = () => {
        if (!destroyed) {
            text.textContent = 'Не удалось загрузить фон карты.';
            status.hidden = false;
        }
    };
    const loadStyle = async () => {
        request?.abort();
        const current = new AbortController();
        request = current;
        status.hidden = true;
        try {
            const response = await fetch(STYLE_URL, { signal: current.signal });
            if (!response.ok) {
                throw new Error('Map style unavailable');
            }
            const style = localizeStyle(
                (await response.json()) as StyleSpecification
            );
            if (!destroyed && !current.signal.aborted) {
                map.setStyle(style, { diff: false });
            }
        } catch {
            if (!current.signal.aborted) {
                showError();
            }
        }
    };
    retry.addEventListener('click', () => void loadStyle());
    status.addEventListener('click', (event) => event.stopPropagation());
    map.on('error', showError);
    const observer = new ResizeObserver(() => map.resize());
    observer.observe(container);
    void loadStyle();
    return {
        map,
        destroy() {
            destroyed = true;
            request?.abort();
            observer.disconnect();
            map.stop();
            map.remove();
            status.remove();
            container.classList.remove(styles.root);
        },
    };
};
