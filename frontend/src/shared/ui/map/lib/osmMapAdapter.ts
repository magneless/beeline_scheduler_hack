import L from 'leaflet';

import { workTypeAppearance } from 'shared/lib/config';

import {
    type MapAdapter,
    type MapMarker,
    type MapPolyline,
    type MapViewProps,
} from './mapContract';
import { createRenderScheduler, mapLayersSignature } from './mapPerf';

import 'leaflet/dist/leaflet.css';

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
const point = (p: { lat: number; lon: number }): L.LatLngExpression => [
    p.lat,
    p.lon,
];
const allPoints = (p: MapViewProps) =>
    [
        ...p.markers.map((m) => m.point),
        ...p.polylines.flatMap((l) => l.points),
    ].filter((p) => Number.isFinite(p.lat) && Number.isFinite(p.lon));
const retryStatus = (container: HTMLElement, retry: () => void) => {
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
const markerVisual = (m: MapMarker, selected: boolean) => {
    const color =
        m.color ??
        (m.muted
            ? '#8a918e'
            : m.open
              ? '#d49424'
              : (COLORS[m.tone ?? m.kind] ?? COLORS.office));
    const size = m.kind === 'office' ? 28 : m.sequence ? 28 : m.muted ? 10 : 14;
    const icon = document.createElement('div');
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
    return L.divIcon({
        className: 'osm-marker-icon',
        html: icon,
        iconSize: [size, size],
        iconAnchor: [size / 2, size / 2],
    });
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
            markers = new Map<string, L.Marker>(),
            lines = new Map<
                string,
                { casing: L.Polyline; stroke: L.Polyline }
            >();
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
        const fit = () => {
            const pts = allPoints(props);
            if (pts.length) {
                map.fitBounds(L.latLngBounds(pts.map(point)), {
                    padding: [38, 38],
                    maxZoom: 15,
                });
            }
        };
        L.control
            .zoom({
                position: 'topright',
                zoomInTitle: 'Приблизить',
                zoomOutTitle: 'Отдалить',
            })
            .addTo(map);
        const fitControl = new L.Control({ position: 'topright' });
        fitControl.onAdd = () => {
            const button = L.DomUtil.create(
                'button',
                'leaflet-control-route-fit'
            );
            button.type = 'button';
            button.textContent = 'Показать маршрут';
            button.title = 'Показать весь маршрут';
            button.setAttribute('aria-label', button.title);
            L.DomEvent.disableClickPropagation(button);
            L.DomEvent.on(button, 'click keydown', (event: Event) => {
                if (
                    event.type === 'click' ||
                    (event as KeyboardEvent).key === 'Enter' ||
                    (event as KeyboardEvent).key === ' '
                ) {
                    event.preventDefault();
                    fit();
                }
            });
            return button;
        };
        fitControl.addTo(map);
        L.control.scale({ position: 'bottomleft', imperial: false }).addTo(map);
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
            tileErrors += 1;
        });
        tiles.on('load', () => {
            container.querySelector('.osmMapStatus')?.remove();
            if (tileErrors) {
                retryStatus(container, () => {
                    container.querySelector('.osmMapStatus')?.remove();
                    tileErrors = 0;
                    tiles.redraw();
                });
            }
        });
        tiles.addTo(map);
        layers.addTo(map);
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
                    layer = L.marker(point(m.point), {
                        icon: markerVisual(m, selected),
                        keyboard: false,
                        interactive: m.kind === 'order',
                    });
                    layer.on('click', (e) => {
                        L.DomEvent.stopPropagation(e.originalEvent);
                        props.onMarkerClick?.(m.id);
                    });
                    layer.addTo(layers);
                    markers.set(m.id, layer);
                    layer.getElement()?.addEventListener('keydown', (event) => {
                        if (
                            (event as KeyboardEvent).key === 'Enter' ||
                            (event as KeyboardEvent).key === ' '
                        ) {
                            event.preventDefault();
                            event.stopPropagation();
                            props.onMarkerClick?.(m.id);
                        }
                    });
                }
                layer.setLatLng(point(m.point));
                layer.setIcon(markerVisual(m, selected));
                if (
                    selected &&
                    selectedChanged &&
                    !map.getBounds().pad(-0.1).contains(point(m.point))
                ) {
                    map.panTo(point(m.point));
                }
            });
            const lineIds = new Set(props.polylines.map((l) => l.id));
            lines.forEach((pair, id) => {
                if (!lineIds.has(id)) {
                    layers.removeLayer(pair.casing);
                    layers.removeLayer(pair.stroke);
                    lines.delete(id);
                }
            });
            props.polylines.forEach((line: MapPolyline) => {
                let pair = lines.get(line.id);
                const color =
                    line.color ?? COLORS[line.tone ?? 'gold'] ?? COLORS.gold;
                if (!pair) {
                    pair = {
                        casing: L.polyline(line.points.map(point), {
                            interactive: false,
                            color: '#fff',
                            weight: 9,
                            opacity: 0.96,
                            lineCap: 'round',
                            lineJoin: 'round',
                        }).addTo(layers),
                        stroke: L.polyline(line.points.map(point), {
                            interactive: false,
                            color,
                            weight: 5,
                            opacity: 0.94,
                            lineCap: 'round',
                            lineJoin: 'round',
                        }).addTo(layers),
                    };
                    pair.casing
                        .getElement()
                        ?.setAttribute('data-route-id', line.id);
                    pair.stroke
                        .getElement()
                        ?.setAttribute('data-route-id', line.id);
                    lines.set(line.id, pair);
                }
                pair.casing.setLatLngs(line.points.map(point));
                pair.stroke.setLatLngs(line.points.map(point));
                pair.stroke.setStyle({ color, weight: 5, opacity: 0.94 });
            });
            markers.forEach((layer, id) => {
                const marker = props.markers.find((item) => item.id === id);
                layer.setZIndexOffset(
                    marker?.id === props.selectedId
                        ? 1000
                        : marker?.kind === 'office'
                          ? 500
                          : 0
                );
            });
            const fitButton = fitControl.getContainer();
            if (fitButton) {
                const label = props.polylines.length
                    ? 'Показать весь маршрут'
                    : 'Показать все заявки';
                fitButton.textContent = props.polylines.length
                    ? 'Весь маршрут'
                    : 'Все заявки';
                fitButton.title = label;
                fitButton.setAttribute('aria-label', label);
            }
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
