import {
    type MapAdapter,
    type MapPoint,
    type MapViewProps,
} from './mapContract';
import { createRenderScheduler, mapLayersSignature } from './mapPerf';

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

type GeoBounds = {
    minLat: number;
    maxLat: number;
    minLon: number;
    maxLon: number;
};

const boundsFromPoints = (points: MapPoint[]): GeoBounds => {
    const lats = points.map((point) => point.lat);
    const lons = points.map((point) => point.lon);

    return {
        minLat: Math.min(...lats),
        maxLat: Math.max(...lats),
        minLon: Math.min(...lons),
        maxLon: Math.max(...lons),
    };
};

const project = (bounds: GeoBounds, width: number, height: number) => {
    const pad = 0.012;
    const latSpan = Math.max(bounds.maxLat - bounds.minLat, 0.01) + pad * 2;
    const lonSpan = Math.max(bounds.maxLon - bounds.minLon, 0.01) + pad * 2;
    const padX = width * 0.22;
    const padY = height * 0.18;

    return (point: MapPoint) => ({
        x:
            padX +
            ((point.lon - (bounds.minLon - pad)) / lonSpan) *
                (width - padX * 2),
        y:
            padY +
            (1 - (point.lat - (bounds.minLat - pad)) / latSpan) *
                (height - padY * 2),
    });
};

export const createPreviewMapAdapter = (): MapAdapter => ({
    id: 'preview',
    mount(container, initial) {
        let props = initial;
        let lastLayers = '';
        let lastSelected = initial.selectedId;
        let lastFitToken = initial.fitToken;
        let lockedBounds: GeoBounds | null = null;
        const scheduler = createRenderScheduler();
        const resizeScheduler = createRenderScheduler();

        const draw = () => {
            const width = container.clientWidth || 800;
            const height = container.clientHeight || 560;
            const allPoints = [
                ...props.polylines.flatMap((line) => line.points),
                ...props.markers.map((marker) => marker.point),
            ];

            if (props.fitToken !== lastFitToken) {
                lockedBounds = null;
                lastFitToken = props.fitToken;
            }

            if (!lockedBounds && allPoints.length > 0) {
                lockedBounds = boundsFromPoints(allPoints);
            }

            const toXy = project(
                lockedBounds ??
                    boundsFromPoints(
                        allPoints.length
                            ? allPoints
                            : [{ lat: 55.75, lon: 37.62 }]
                    ),
                width,
                height
            );

            const svg = document.createElementNS(
                'http://www.w3.org/2000/svg',
                'svg'
            );
            svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
            svg.setAttribute('width', '100%');
            svg.setAttribute('height', '100%');
            svg.style.display = 'block';

            const backdrop = document.createElementNS(
                'http://www.w3.org/2000/svg',
                'rect'
            );
            backdrop.setAttribute('width', String(width));
            backdrop.setAttribute('height', String(height));
            backdrop.setAttribute('fill', '#f7f7f5');
            svg.append(backdrop);

            for (let step = 0; step < 12; step += 1) {
                const line = document.createElementNS(
                    'http://www.w3.org/2000/svg',
                    'path'
                );
                const y = (height / 12) * step;
                const x = (width / 12) * step;
                line.setAttribute('d', `M0 ${y} H${width} M${x} 0 V${height}`);
                line.setAttribute('stroke', 'rgba(26,26,26,0.05)');
                line.setAttribute('stroke-width', '1');
                svg.append(line);
            }

            props.polylines.forEach((polyline) => {
                if (polyline.points.length < 2) {
                    return;
                }

                const path = document.createElementNS(
                    'http://www.w3.org/2000/svg',
                    'path'
                );
                const d = polyline.points
                    .map((point, index) => {
                        const { x, y } = toXy(point);
                        return `${index === 0 ? 'M' : 'L'}${x} ${y}`;
                    })
                    .join(' ');
                path.setAttribute('d', d);
                path.setAttribute('stroke', TONE[polyline.tone ?? 'gold']);
                path.setAttribute('stroke-width', '4');
                path.setAttribute('fill', 'none');
                path.setAttribute('stroke-linecap', 'round');
                path.setAttribute('stroke-linejoin', 'round');
                svg.append(path);
            });

            props.markers.forEach((marker) => {
                const { x, y } = toXy(marker.point);
                const selected = props.selectedId === marker.id;
                const group = document.createElementNS(
                    'http://www.w3.org/2000/svg',
                    'g'
                );
                const fill = TONE[marker.tone ?? marker.kind];
                group.style.cursor = 'pointer';
                group.addEventListener('click', (event) => {
                    event.stopPropagation();
                    props.onMarkerClick?.(marker.id);
                });

                if (selected) {
                    const ring = document.createElementNS(
                        'http://www.w3.org/2000/svg',
                        'circle'
                    );
                    ring.setAttribute('cx', String(x));
                    ring.setAttribute('cy', String(y));
                    ring.setAttribute('r', '14');
                    ring.setAttribute('fill', 'none');
                    ring.setAttribute('stroke', fill);
                    ring.setAttribute('stroke-width', '2');
                    ring.setAttribute('opacity', '0.45');
                    group.append(ring);
                }

                const circle = document.createElementNS(
                    'http://www.w3.org/2000/svg',
                    'circle'
                );
                circle.setAttribute('cx', String(x));
                circle.setAttribute('cy', String(y));
                circle.setAttribute('r', selected ? '8' : '7');
                circle.setAttribute('fill', fill);
                circle.setAttribute('stroke', '#fff');
                circle.setAttribute('stroke-width', '2');
                group.append(circle);

                if (marker.label) {
                    const label = document.createElementNS(
                        'http://www.w3.org/2000/svg',
                        'text'
                    );
                    label.setAttribute('x', String(x + 12));
                    label.setAttribute('y', String(y + 4));
                    label.setAttribute('fill', '#1a1a1a');
                    label.setAttribute('font-size', '11');
                    label.setAttribute('font-weight', '600');
                    label.textContent = marker.label;
                    group.append(label);
                }

                svg.append(group);
            });

            container.replaceChildren(svg);
            lastLayers = mapLayersSignature(props);
            lastSelected = props.selectedId;
        };

        const queueDraw = (force = false) => {
            scheduler.schedule(() => {
                const layers = mapLayersSignature(props);
                const selectionChanged = props.selectedId !== lastSelected;
                const tokenChanged = props.fitToken !== lastFitToken;

                if (
                    !force &&
                    layers === lastLayers &&
                    !selectionChanged &&
                    !tokenChanged
                ) {
                    return;
                }

                draw();
            });
        };

        draw();
        const observer = new ResizeObserver(() => {
            resizeScheduler.schedule(() => draw());
        });
        observer.observe(container);

        return {
            update: (next: MapViewProps) => {
                props = next;
                queueDraw();
            },
            destroy: () => {
                scheduler.cancel();
                resizeScheduler.cancel();
                observer.disconnect();
            },
        };
    },
});
