import {
    type MapMarker,
    type MapPolyline,
    type MapViewProps,
} from './mapContract';

export const markerGeometrySignature = (marker: MapMarker) =>
    [
        marker.id,
        marker.point.lat,
        marker.point.lon,
        marker.kind,
        marker.open ? 1 : 0,
        marker.tone ?? '',
        marker.label ?? '',
    ].join('|');

export const polylineGeometrySignature = (line: MapPolyline) => {
    const { points } = line;
    const head = points[0];
    const tail = points[points.length - 1];

    return [
        line.id,
        line.tone ?? '',
        points.length,
        head ? `${head.lat},${head.lon}` : '',
        tail ? `${tail.lat},${tail.lon}` : '',
    ].join('|');
};

export const mapLayersSignature = (props: MapViewProps) => {
    const markers = props.markers.map(markerGeometrySignature).join(';');
    const lines = props.polylines.map(polylineGeometrySignature).join(';');

    return `${markers}#${lines}`;
};

export const createRenderScheduler = () => {
    let frame = 0;
    let latest: (() => void) | null = null;

    const schedule = (task: () => void) => {
        latest = task;

        if (frame) {
            return;
        }

        frame = requestAnimationFrame(() => {
            frame = 0;
            const run = latest;
            latest = null;
            run?.();
        });
    };

    const cancel = () => {
        if (frame) {
            cancelAnimationFrame(frame);
            frame = 0;
        }

        latest = null;
    };

    return { schedule, cancel };
};
