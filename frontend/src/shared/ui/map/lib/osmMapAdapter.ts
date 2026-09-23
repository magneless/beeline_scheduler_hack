import L from 'leaflet';

import {
    type MapAdapter,
    type MapMarker,
    type MapPolyline,
    type MapViewProps,
} from './mapContract';
import { createRenderScheduler, mapLayersSignature } from './mapPerf';

import 'leaflet/dist/leaflet.css';

const COLORS: Record<string, string> = {
    office: '#ffcc00',
    emergency: '#e30611',
    connection: '#ffcc00',
    repair: '#1a1a1a',
    additional: '#a3a39a',
    gold: '#ffcc00',
    ice: '#1a1a1a',
    lime: '#c4a000',
};
const latLng = (p: { lat: number; lon: number }): L.LatLngExpression => [
    p.lat,
    p.lon,
];
const allPoints = (p: MapViewProps) =>
    [
        ...p.markers.map((x) => x.point),
        ...p.polylines.flatMap((x) => x.points),
    ].filter((x) => Number.isFinite(x.lat) && Number.isFinite(x.lon));
const status = (container: HTMLElement, retry: () => void) => {
    const note = document.createElement('div');
    note.className = 'osmMapStatus';
    note.setAttribute('role', 'status');
    L.DomEvent.disableClickPropagation(note);
    L.DomEvent.disableScrollPropagation(note);
    const text = document.createElement('span');
    text.textContent = 'Не удалось загрузить карту OpenStreetMap.';
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = 'Повторить';
    button.addEventListener('click', retry);
    note.append(text, button);
    container.append(note);
};
const style = (m: MapMarker, selected: boolean): L.CircleMarkerOptions => {
    const color = COLORS[m.tone ?? m.kind] ?? COLORS.office;
    return {
        color: m.kind === 'office' ? '#1a1a1a' : selected ? '#fff' : color,
        fillColor: color,
        fillOpacity: 0.9,
        weight: selected ? 4 : m.kind === 'office' ? 3 : 2,
        radius: m.kind === 'office' ? (selected ? 12 : 10) : selected ? 9 : 7,
    };
};

export const createOsmMapAdapter = (): MapAdapter => ({
    id: 'osm',
    mount(container, initial) {
        let props = initial,
            destroyed = false,
            didFit = false,
            lastFit = initial.fitToken,
            lastSignature = '',
            lastSelected = initial.selectedId,
            tileErrors = 0;
        const scheduler = createRenderScheduler(),
            layers = L.layerGroup(),
            markers = new Map<string, L.CircleMarker>(),
            lines = new Map<string, L.Polyline>();
        container.classList.add('osm-map');
        const map = L.map(container, {
            center: [55.75, 37.62],
            zoom: 11,
            zoomControl: false,
            attributionControl: false,
            scrollWheelZoom: true,
            dragging: true,
            keyboard: true,
            touchZoom: true,
        });
        L.control
            .zoom({
                position: 'topleft',
                zoomInTitle: 'Приблизить',
                zoomOutTitle: 'Отдалить',
            })
            .addTo(map);
        const tiles = L.tileLayer(
            'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
            {
                maxZoom: 19,
                attribution:
                    '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
            }
        );
        tiles.on('loading', () => {
            tileErrors = 0;
        });
        tiles.on('tileerror', () => {
            tileErrors++;
        });
        tiles.on('load', () => {
            container.querySelector('.osmMapStatus')?.remove();
            if (tileErrors > 0) {
                status(container, () => {
                    container.querySelector('.osmMapStatus')?.remove();
                    tileErrors = 0;
                    tiles.redraw();
                });
            }
        });
        tiles.addTo(map);
        layers.addTo(map);
        const fit = () => {
            const pts = allPoints(props);
            if (pts.length) {
                map.fitBounds(L.latLngBounds(pts.map(latLng)), {
                    padding: [32, 32],
                    maxZoom: 15,
                });
            }
        };
        const sync = () => {
            if (destroyed) {
                return;
            }
            const sig = mapLayersSignature(props),
                fitChanged = props.fitToken !== lastFit,
                selectedChanged = props.selectedId !== lastSelected;
            if (sig === lastSignature && !fitChanged && !selectedChanged) {
                return;
            }
            if ((!didFit || fitChanged) && allPoints(props).length) {
                fit();
                didFit = true;
                lastFit = props.fitToken;
            }
            const ids = new Set(props.markers.map((m) => m.id));
            markers.forEach((layer, id) => {
                if (!ids.has(id)) {
                    layers.removeLayer(layer);
                    markers.delete(id);
                }
            });
            props.markers.forEach((m) => {
                const selected = props.selectedId === m.id;
                let layer = markers.get(m.id);
                if (!layer) {
                    layer = L.circleMarker(latLng(m.point), style(m, selected));
                    layer.on('click', (e) => {
                        L.DomEvent.stopPropagation(e.originalEvent);
                        props.onMarkerClick?.(m.id);
                    });
                    layer.addTo(layers);
                    markers.set(m.id, layer);
                    layer.getElement()?.addEventListener('keydown', (event) => {
                        if (
                            event instanceof KeyboardEvent &&
                            (event.key === 'Enter' || event.key === ' ')
                        ) {
                            event.preventDefault();
                            event.stopPropagation();
                            props.onMarkerClick?.(m.id);
                        }
                    });
                }
                layer.setLatLng(latLng(m.point));
                layer.setStyle(style(m, selected));
                const element = layer.getElement();
                element?.setAttribute('data-marker-id', m.id);
                element?.setAttribute(
                    'role',
                    m.kind === 'office' ? 'img' : 'button'
                );
                element?.setAttribute(
                    'aria-label',
                    m.kind === 'office'
                        ? 'Офис'
                        : `Заявка ${m.id}: ${m.label ?? ''}`
                );
                if (m.kind === 'order') {
                    element?.setAttribute('tabindex', '0');
                    element?.setAttribute('aria-pressed', String(selected));
                }
                if (m.label) {
                    const label = document.createElement('span');
                    label.textContent = m.label;
                    layer.bindTooltip(label, {
                        direction: 'top',
                        offset: [0, -7],
                    });
                } else {
                    layer.unbindTooltip();
                }
                if (
                    selected &&
                    selectedChanged &&
                    !map.getBounds().pad(-0.1).contains(latLng(m.point))
                ) {
                    map.panTo(latLng(m.point));
                }
            });
            const lineIds = new Set(props.polylines.map((x) => x.id));
            lines.forEach((layer, id) => {
                if (!lineIds.has(id)) {
                    layers.removeLayer(layer);
                    lines.delete(id);
                }
            });
            props.polylines.forEach((line: MapPolyline) => {
                let layer = lines.get(line.id);
                if (!layer) {
                    layer = L.polyline(line.points.map(latLng), {
                        interactive: false,
                        color: COLORS[line.tone ?? 'gold'] ?? COLORS.gold,
                        weight: 4,
                        opacity: 0.85,
                        lineCap: 'round',
                        lineJoin: 'round',
                    }).addTo(layers);
                    lines.set(line.id, layer);
                }
                layer.setLatLngs(line.points.map(latLng));
                layer.setStyle({
                    color: COLORS[line.tone ?? 'gold'] ?? COLORS.gold,
                });
            });
            markers.forEach((layer) => layer.bringToFront());
            lastSignature = sig;
            lastSelected = props.selectedId;
        };
        sync();
        const observer = new ResizeObserver(() =>
            map.invalidateSize({ pan: false })
        );
        observer.observe(container);
        return {
            update(next) {
                props = next;
                scheduler.schedule(sync);
            },
            destroy() {
                destroyed = true;
                scheduler.cancel();
                observer.disconnect();
                container.querySelector('.osmMapStatus')?.remove();
                map.remove();
                container.classList.remove('osm-map');
                markers.clear();
                lines.clear();
            },
        };
    },
});
