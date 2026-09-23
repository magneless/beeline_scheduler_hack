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
        marker.color ?? '',
        marker.sequence ?? '',
        marker.muted ? 1 : 0,
    ].join('|');

export const polylineGeometrySignature = (line: MapPolyline) =>
    [
        line.id,
        line.tone ?? '',
        line.color ?? '',
        ...line.points.map(({ lat, lon }) => `${lat},${lon}`),
    ].join('|');

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
