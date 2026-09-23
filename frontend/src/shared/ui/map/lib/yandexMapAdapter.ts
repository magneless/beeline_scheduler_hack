import { env } from 'shared/config/env';
import { type TypeOrNull } from 'shared/lib/types';

import {
    type MapAdapter,
    type MapMarker,
    type MapPolyline,
    type MapViewProps,
} from './mapContract';
import {
    createRenderScheduler,
    mapLayersSignature,
    markerGeometrySignature,
    polylineGeometrySignature,
} from './mapPerf';

type LatLng = [number, number];

type YMap = {
    geoObjects: {
        add: (object: unknown) => void;
        remove: (object: unknown) => void;
    };
    controls: { add: (name: string, options?: object) => void };
    options: { set: (key: string, value: unknown) => void };
    setBounds: (bounds: LatLng[], options?: object) => void;
    container: { fitToViewport: () => void };
    destroy: () => void;
};

type YCollection = {
    add: (object: unknown) => void;
    remove: (object: unknown) => void;
    removeAll: () => void;
    getBounds: () => TypeOrNull<LatLng[]>;
};

type YOptionTarget = {
    options: { set: (key: string, value: unknown) => void };
    properties: { set: (key: string, value: unknown) => void };
    geometry: { setCoordinates: (coords: LatLng | LatLng[]) => void };
    events: { add: (name: string, handler: () => void) => void };
};

type YMapsApi = {
    ready: (callback: () => void) => void;
    Map: new (
        container: HTMLElement,
        state: { center: LatLng; zoom: number; controls: string[] },
        options?: object
    ) => YMap;
    Placemark: new (
        geometry: LatLng,
        properties?: object,
        options?: object
    ) => YOptionTarget;
    Polyline: new (
        geometry: LatLng[],
        properties?: object,
        options?: object
    ) => YOptionTarget;
    GeoObjectCollection: new () => YCollection;
};

const TONE: Record<string, string> = {
    office: '#ffcc00',
    emergency: '#e30611',
    connection: '#ffcc00',
    repair: '#1a1a1a',
    additional: '#a3a39a',
    gold: '#ffcc00',
    ice: '#1a1a1a',
    lime: '#c4a000',
};

let ymapsLoader: TypeOrNull<Promise<YMapsApi>> = null;

const loadYmaps = () => {
    const fromWindow = (window as Window & { ymaps?: YMapsApi }).ymaps;

    if (fromWindow) {
        return new Promise<YMapsApi>((resolve) => {
            fromWindow.ready(() => resolve(fromWindow));
        });
    }

    if (ymapsLoader) {
        return ymapsLoader;
    }

    ymapsLoader = new Promise((resolve, reject) => {
        const script = document.createElement('script');
        const params = new URLSearchParams({ lang: 'ru_RU' });

        if (env.yandexMapsKey) {
            params.set('apikey', env.yandexMapsKey);
        }

        script.src = `https://api-maps.yandex.ru/2.1/?${params.toString()}`;
        script.async = true;
        script.onload = () => {
            const ymaps = (window as Window & { ymaps?: YMapsApi }).ymaps;

            if (!ymaps) {
                reject(new Error('Яндекс.Карты не инициализировались'));
                return;
            }

            ymaps.ready(() => resolve(ymaps));
        };
        script.onerror = () =>
            reject(new Error('Не удалось загрузить Яндекс.Карты'));
        document.head.append(script);
    });

    return ymapsLoader;
};

const showMessage = (container: HTMLElement, text: string) => {
    container.replaceChildren();
    const note = document.createElement('div');
    note.style.display = 'flex';
    note.style.alignItems = 'center';
    note.style.justifyContent = 'center';
    note.style.height = '100%';
    note.style.padding = '24px';
    note.style.color = '#73736c';
    note.style.fontSize = '15px';
    note.style.textAlign = 'center';
    note.textContent = text;
    container.append(note);
};

const markerPreset = (marker: MapMarker, selected: boolean) => {
    if (selected) {
        return 'islands#dotIcon';
    }

    if (marker.open || marker.kind === 'office') {
        return 'islands#circleDotIcon';
    }

    return 'islands#circleIcon';
};

const applyMarkerStyle = (
    placemark: YOptionTarget,
    marker: MapMarker,
    selected: boolean
) => {
    const color = TONE[marker.tone ?? marker.kind];

    placemark.options.set('preset', markerPreset(marker, selected));
    placemark.options.set('iconColor', color);
    placemark.options.set('zIndex', selected ? 700 : 1);
    placemark.properties.set('iconCaption', marker.label ?? '');
    placemark.properties.set('hintContent', marker.label ?? '');
};

export const createYandexMapAdapter = (): MapAdapter => ({
    id: 'yandex',
    mount(container, initial) {
        let destroyed = false;
        let map: TypeOrNull<YMap> = null;
        let collection: TypeOrNull<YCollection> = null;
        let ymapsApi: TypeOrNull<YMapsApi> = null;
        let pending = initial;
        let didFit = false;
        let lastFitToken = initial.fitToken;
        let lastLayers = '';
        let lastSelected: TypeOrNull<string> | undefined = initial.selectedId;
        const markers = new Map<
            string,
            { object: YOptionTarget; signature: string }
        >();
        const lines = new Map<
            string,
            { object: YOptionTarget; signature: string }
        >();
        const scheduler = createRenderScheduler();
        const resizeScheduler = createRenderScheduler();

        const syncMarkers = (props: MapViewProps, ymaps: YMapsApi) => {
            if (!collection) {
                return;
            }

            const nextIds = new Set(props.markers.map((marker) => marker.id));

            markers.forEach((entry, id) => {
                if (nextIds.has(id)) {
                    return;
                }

                collection?.remove(entry.object);
                markers.delete(id);
            });

            props.markers.forEach((marker) => {
                const signature = markerGeometrySignature(marker);
                const selected = props.selectedId === marker.id;
                const existing = markers.get(marker.id);

                if (!existing) {
                    const placemark = new ymaps.Placemark(
                        [marker.point.lat, marker.point.lon],
                        {
                            iconCaption: marker.label ?? '',
                            hintContent: marker.label ?? '',
                        },
                        {
                            preset: markerPreset(marker, selected),
                            iconColor: TONE[marker.tone ?? marker.kind],
                            zIndex: selected ? 700 : 1,
                        }
                    );

                    placemark.events.add('click', () => {
                        pending.onMarkerClick?.(marker.id);
                    });
                    collection?.add(placemark);
                    markers.set(marker.id, { object: placemark, signature });
                    return;
                }

                if (existing.signature !== signature) {
                    existing.object.geometry.setCoordinates([
                        marker.point.lat,
                        marker.point.lon,
                    ]);
                    existing.signature = signature;
                }

                applyMarkerStyle(existing.object, marker, selected);
            });
        };

        const syncLines = (props: MapViewProps, ymaps: YMapsApi) => {
            if (!collection) {
                return;
            }

            const nextIds = new Set(props.polylines.map((line) => line.id));

            lines.forEach((entry, id) => {
                if (nextIds.has(id)) {
                    return;
                }

                collection?.remove(entry.object);
                lines.delete(id);
            });

            props.polylines.forEach((line: MapPolyline) => {
                if (line.points.length < 2) {
                    return;
                }

                const signature = polylineGeometrySignature(line);
                const path = line.points.map(
                    (point) => [point.lat, point.lon] as LatLng
                );
                const color = TONE[line.tone ?? 'gold'];
                const existing = lines.get(line.id);

                if (!existing) {
                    const polyline = new ymaps.Polyline(
                        path,
                        {},
                        {
                            strokeColor: color,
                            strokeWidth: 5,
                            strokeOpacity: 0.92,
                            strokeStyle: 'solid',
                        }
                    );
                    collection?.add(polyline);
                    lines.set(line.id, { object: polyline, signature });
                    return;
                }

                if (existing.signature === signature) {
                    return;
                }

                existing.object.geometry.setCoordinates(path);
                existing.object.options.set('strokeColor', color);
                existing.signature = signature;
            });
        };

        const fitIfNeeded = (layersChanged: boolean) => {
            if (!map || !collection || !layersChanged || didFit) {
                return;
            }

            const bounds = collection.getBounds();

            if (!bounds) {
                return;
            }

            map.setBounds(bounds, {
                checkZoomRange: true,
                duration: 0,
                zoomMargin: [96, 48, 48, 48],
            });
            didFit = true;
        };

        const apply = (props: MapViewProps) => {
            if (!map || !collection || !ymapsApi) {
                return;
            }

            const tokenChanged = props.fitToken !== lastFitToken;

            if (tokenChanged) {
                didFit = false;
                lastFitToken = props.fitToken;
            }

            const nextLayers = mapLayersSignature(props);
            const layersChanged = nextLayers !== lastLayers;
            const selectionChanged = props.selectedId !== lastSelected;

            if (!layersChanged && !selectionChanged && !tokenChanged) {
                return;
            }

            if (layersChanged || tokenChanged) {
                syncLines(props, ymapsApi);
                syncMarkers(props, ymapsApi);
                lastLayers = nextLayers;
                fitIfNeeded(true);
            } else {
                props.markers.forEach((marker) => {
                    const entry = markers.get(marker.id);

                    if (!entry) {
                        return;
                    }

                    applyMarkerStyle(
                        entry.object,
                        marker,
                        props.selectedId === marker.id
                    );
                });
            }

            lastSelected = props.selectedId;
        };

        const queueApply = (props: MapViewProps) => {
            pending = props;
            scheduler.schedule(() => {
                if (!destroyed) {
                    apply(pending);
                }
            });
        };

        showMessage(container, 'Загружаем Яндекс.Карты…');

        void loadYmaps()
            .then((ymaps) => {
                if (destroyed) {
                    return;
                }

                ymapsApi = ymaps;
                container.replaceChildren();
                map = new ymaps.Map(
                    container,
                    {
                        center: [55.7558, 37.6173],
                        zoom: 12,
                        controls: [],
                    },
                    {
                        suppressMapOpenBlock: true,
                        yandexMapDisablePoiInteractivity: true,
                        copyrightLogoVisible: false,
                        copyrightProvidersVisible: false,
                        copyrightUaVisible: false,
                    }
                );
                map.options.set('copyrightLogoVisible', false);
                map.options.set('copyrightProvidersVisible', false);
                map.options.set('copyrightUaVisible', false);
                map.controls.add('zoomControl', {
                    size: 'small',
                    position: { left: 16, bottom: 24 },
                });
                collection = new ymaps.GeoObjectCollection();
                map.geoObjects.add(collection);
                apply(pending);
                map.container.fitToViewport();
            })
            .catch(() => {
                if (!destroyed) {
                    showMessage(
                        container,
                        env.yandexMapsKey
                            ? 'Яндекс.Карты не загрузились. Проверьте ключ VITE_YANDEX_MAPS_KEY.'
                            : 'Нужен бесплатный ключ JS API Яндекс.Карт в VITE_YANDEX_MAPS_KEY.'
                    );
                }
            });

        const observer = new ResizeObserver(() => {
            resizeScheduler.schedule(() => {
                map?.container.fitToViewport();
            });
        });
        observer.observe(container);

        return {
            update: queueApply,
            destroy: () => {
                destroyed = true;
                scheduler.cancel();
                resizeScheduler.cancel();
                observer.disconnect();
                markers.clear();
                lines.clear();
                map?.destroy();
                map = null;
                collection = null;
                ymapsApi = null;
            },
        };
    },
});
