import { useEffect, useRef } from 'react';
import { type Map as VectorMap, Marker } from 'maplibre-gl';

import { type Point } from 'shared/api/types/contracts';
import { createVectorMap } from 'shared/ui/map/lib/createVectorMap';

type Props = {
    center: Point;
    focus?: Point;
    point?: Point;
    onPoint: (point: Point) => void;
};

export const AddressPointPicker = ({
    center,
    focus,
    point,
    onPoint,
}: Props) => {
    const container = useRef<HTMLDivElement>(null);
    const map = useRef<VectorMap | null>(null);
    const marker = useRef<Marker | null>(null);
    const onPick = useRef(onPoint);
    onPick.current = onPoint;
    const initial = useRef(center);
    useEffect(() => {
        if (!container.current) {
            return;
        }
        const handle = createVectorMap(container.current, initial.current, 14);
        map.current = handle.map;
        handle.map?.on('click', (event) => {
            onPick.current({ lat: event.lngLat.lat, lon: event.lngLat.lng });
        });
        return () => {
            marker.current?.remove();
            marker.current = null;
            map.current = null;
            handle.destroy();
        };
    }, []);
    useEffect(() => {
        if (focus) {
            map.current?.jumpTo({ center: [focus.lon, focus.lat], zoom: 17 });
        }
    }, [focus]);
    useEffect(() => {
        if (!map.current) {
            return;
        }
        if (!point) {
            marker.current?.remove();
            marker.current = null;
            return;
        }
        const position: [number, number] = [point.lon, point.lat];
        if (marker.current) {
            marker.current.setLngLat(position);
        } else {
            marker.current = new Marker({ color: '#ffcc00', draggable: true })
                .setLngLat(position)
                .addTo(map.current);
            marker.current
                .getElement()
                .setAttribute('aria-label', 'Выбранный дом');
            marker.current.on('dragend', () => {
                const value = marker.current?.getLngLat();
                if (value) {
                    onPick.current({ lat: value.lat, lon: value.lng });
                }
            });
        }
        map.current.panTo(position, { duration: 0 });
    }, [point]);
    return (
        <div
            ref={container}
            className="relative z-0 h-64 w-full overflow-hidden rounded-lg border"
            aria-label="Выберите дом на карте"
        />
    );
};
