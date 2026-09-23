import { type TypeOrNull } from 'shared/lib/types';

export type MapPoint = {
    lat: number;
    lon: number;
};

export type MapMarker = {
    id: string;
    point: MapPoint;
    kind: 'office' | 'order';
    selected?: boolean;
    open?: boolean;
    label?: string;
    tone?: 'emergency' | 'connection' | 'repair' | 'additional' | 'office';
    color?: string;
    sequence?: number;
    muted?: boolean;
};

export type MapPolyline = {
    id: string;
    points: MapPoint[];
    tone?: 'gold' | 'ice' | 'lime';
    color?: string;
};

export type MapViewProps = {
    markers: MapMarker[];
    polylines: MapPolyline[];
    selectedId?: TypeOrNull<string>;
    fitToken?: string;
    onMarkerClick?: (id: string) => void;
};

export type MapHandle = {
    update: (props: MapViewProps) => void;
    destroy: () => void;
};

export type MapAdapter = {
    id: 'yandex' | 'osm' | 'preview';
    mount: (container: HTMLElement, props: MapViewProps) => MapHandle;
};
