import { type GeoJSONSource, LngLatBounds, Marker } from 'maplibre-gl';

import { workTypeAppearance } from 'shared/lib/config';

import { createVectorMap } from './createVectorMap';
import {
    type MapAdapter,
    type MapMarker,
    type MapViewProps,
} from './mapContract';
import { createRenderScheduler, mapLayersSignature } from './mapPerf';

const COLORS: Record<string, string> = {
    office: '#27332f',
    emergency: workTypeAppearance.emergency.background,
    connection: workTypeAppearance.connection.background,
    repair: workTypeAppearance.repair.background,
    additional: workTypeAppearance.additional.background,
    gold: '#277a62',
    ice: '#3578c4',
    lime: '#a57716',
};
const updateMarkerVisual = (
    icon: HTMLElement,
    m: MapMarker,
    selected: boolean
) => {
    const color =
        m.color ??
        (m.muted
            ? '#8a918e'
            : m.open
              ? '#d49424'
              : (COLORS[m.tone ?? m.kind] ?? COLORS.office));
    const size = m.kind === 'office' ? 28 : m.sequence ? 28 : m.muted ? 10 : 14;
    icon.className = [
        'osm-marker',
        m.kind === 'office' ? 'osm-office' : '',
        m.muted ? 'osm-marker-muted' : '',
        m.open ? 'osm-marker-open' : '',
        selected ? 'osm-marker-selected' : '',
    ]
        .filter(Boolean)
        .join(' ');
    icon.style.setProperty('--marker-color', color);
    icon.style.setProperty(
        '--marker-foreground',
        m.sequence && m.tone === 'connection'
            ? workTypeAppearance.connection.foreground
            : '#ffffff'
    );
    icon.style.width = `${size}px`;
    icon.style.height = `${size}px`;
    icon.textContent =
        m.kind === 'office' ? 'О' : m.sequence ? String(m.sequence) : '';
    icon.title = m.kind === 'office' ? 'Офис' : (m.label ?? 'Остановка');
    icon.setAttribute('aria-label', icon.title);
    icon.setAttribute('data-marker-id', m.id);
    icon.setAttribute('role', m.kind === 'office' ? 'img' : 'button');
    if (m.kind === 'order') {
        icon.tabIndex = 0;
        icon.setAttribute('aria-pressed', String(selected));
    }
};

const allPoints = (props: MapViewProps) =>
    [
        ...props.markers.map((marker) => marker.point),
        ...props.polylines.flatMap((line) => line.points),
    ].filter(
        (point) => Number.isFinite(point.lat) && Number.isFinite(point.lon)
    );

const ROUTES = 'dispatch-routes';

export const createOsmMapAdapter = (): MapAdapter => ({
    id: 'osm',
    mount(container, initial) {
        let props = initial;
        let destroyed = false;
        let didFit = false;
        let lastFit = initial.fitToken;
        let lastSignature = '';
        let lastSelected = initial.selectedId;
        let styleReady = false;
        const scheduler = createRenderScheduler();
        const markers = new Map<
            string,
            { marker: Marker; visual: HTMLElement }
        >();
        const handle = createVectorMap(
            container,
            { lat: 55.75, lon: 37.62 },
            11
        );
        const map = handle.map;
        if (!map) {
            return { update() {}, destroy: handle.destroy };
        }
        const fit = () => {
            const points = allPoints(props);
            if (points.length) {
                const bounds = new LngLatBounds();
                points.forEach((point) =>
                    bounds.extend([point.lon, point.lat])
                );
                map.fitBounds(bounds, {
                    padding: 38,
                    maxZoom: 15,
                    duration: 0,
                });
            }
        };
        const fitButton = document.createElement('button');
        fitButton.type = 'button';
        fitButton.className = 'maplibregl-ctrl route-fit';
        fitButton.addEventListener('click', fit);
        map.addControl(
            {
                onAdd: () => fitButton,
                onRemove: () => fitButton.remove(),
            },
            'top-right'
        );
        const sync = () => {
            if (destroyed) {
                return;
            }
            const signature = mapLayersSignature(props);
            const fitChanged = props.fitToken !== lastFit;
            const selectedChanged = props.selectedId !== lastSelected;
            if (
                signature === lastSignature &&
                !fitChanged &&
                !selectedChanged
            ) {
                return;
            }
            if ((!didFit || fitChanged) && allPoints(props).length) {
                fit();
                didFit = true;
                lastFit = props.fitToken;
            }
            const ids = new Set(props.markers.map((item) => item.id));
            markers.forEach((item, id) => {
                if (!ids.has(id)) {
                    item.marker.remove();
                    markers.delete(id);
                }
            });
            props.markers.forEach((item) => {
                const selected = props.selectedId === item.id;
                let entry = markers.get(item.id);
                if (!entry) {
                    const element = document.createElement('div');
                    const visual = document.createElement('div');
                    element.append(visual);
                    const marker = new Marker({ element, anchor: 'center' })
                        .setLngLat([item.point.lon, item.point.lat])
                        .addTo(map);
                    if (item.kind === 'order') {
                        const choose = () => props.onMarkerClick?.(item.id);
                        visual.addEventListener('click', (event) => {
                            event.stopPropagation();
                            choose();
                        });
                        visual.addEventListener('keydown', (event) => {
                            if (event.key === 'Enter' || event.key === ' ') {
                                event.preventDefault();
                                event.stopPropagation();
                                choose();
                            }
                        });
                    }
                    entry = { marker, visual };
                    markers.set(item.id, entry);
                }
                entry.marker.setLngLat([item.point.lon, item.point.lat]);
                updateMarkerVisual(entry.visual, item, selected);
                entry.marker.getElement().style.zIndex = selected
                    ? '10'
                    : item.kind === 'office'
                      ? '5'
                      : '1';
                if (
                    selected &&
                    selectedChanged &&
                    !map.getBounds().contains([item.point.lon, item.point.lat])
                ) {
                    map.panTo([item.point.lon, item.point.lat], {
                        duration: 250,
                    });
                }
            });
            if (styleReady) {
                const data = {
                    type: 'FeatureCollection' as const,
                    features: props.polylines
                        .filter((line) => line.points.length >= 2)
                        .map((line) => ({
                            type: 'Feature' as const,
                            id: line.id,
                            properties: {
                                color:
                                    line.color ??
                                    COLORS[line.tone ?? 'gold'] ??
                                    COLORS.gold,
                            },
                            geometry: {
                                type: 'LineString' as const,
                                coordinates: line.points.map((point) => [
                                    point.lon,
                                    point.lat,
                                ]),
                            },
                        })),
                };
                const source = map.getSource<GeoJSONSource>(ROUTES);
                if (source) {
                    source.setData(data);
                } else {
                    map.addSource(ROUTES, { type: 'geojson', data });
                    map.addLayer({
                        id: ROUTES + '-casing',
                        type: 'line',
                        source: ROUTES,
                        layout: { 'line-cap': 'round', 'line-join': 'round' },
                        paint: {
                            'line-color': '#ffffff',
                            'line-width': 9,
                            'line-opacity': 0.96,
                        },
                    });
                    map.addLayer({
                        id: ROUTES + '-stroke',
                        type: 'line',
                        source: ROUTES,
                        layout: { 'line-cap': 'round', 'line-join': 'round' },
                        paint: {
                            'line-color': ['get', 'color'],
                            'line-width': 5,
                            'line-opacity': 0.94,
                        },
                    });
                }
            } else {
                // A pending style or source load must not swallow new route data.
                return;
            }
            const label = props.polylines.length
                ? 'Показать весь маршрут'
                : 'Показать все заявки';
            fitButton.textContent = props.polylines.length
                ? 'Весь маршрут'
                : 'Все заявки';
            fitButton.title = label;
            fitButton.setAttribute('aria-label', label);
            lastSignature = signature;
            lastSelected = props.selectedId;
        };
        map.on('styledataloading', () => {
            styleReady = false;
        });
        map.on('style.load', () => {
            styleReady = true;
            lastSignature = '';
            scheduler.schedule(sync);
        });
        map.on('data', () => scheduler.schedule(sync));
        sync();
        return {
            update(next) {
                props = next;
                scheduler.schedule(sync);
            },
            destroy() {
                destroyed = true;
                scheduler.cancel();
                markers.forEach((entry) => entry.marker.remove());
                markers.clear();
                handle.destroy();
            },
        };
    },
});
