import { useEffect, useRef } from 'react';

import { type TypeOrNull } from 'shared/lib/types';

import { getMapAdapter } from '../lib/getMapAdapter';
import { type MapHandle, type MapViewProps } from '../lib/mapContract';

import styles from './MapView.module.scss';

export const MapView = ({
    markers,
    polylines,
    selectedId,
    fitToken,
    onMarkerClick,
}: MapViewProps) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const handleRef = useRef<TypeOrNull<MapHandle>>(null);
    const onClickRef = useRef(onMarkerClick);

    onClickRef.current = onMarkerClick;

    useEffect(() => {
        const container = containerRef.current;

        if (!container) {
            return;
        }

        const adapter = getMapAdapter();
        const handle = adapter.mount(container, {
            markers,
            polylines,
            selectedId,
            fitToken,
            onMarkerClick: (id) => onClickRef.current?.(id),
        });
        handleRef.current = handle;

        return () => {
            handle.destroy();
            handleRef.current = null;
        };
        // Map instance stays alive; layers update in the effect below.
    }, []);

    useEffect(() => {
        handleRef.current?.update({
            markers,
            polylines,
            selectedId,
            fitToken,
            onMarkerClick: (id) => onClickRef.current?.(id),
        });
    }, [markers, polylines, selectedId, fitToken]);

    return (
        <div className={styles.root}>
            <div ref={containerRef} className={styles.canvas} />
            <div className={styles.attribution}>
                Карта и маршруты: ©{' '}
                <a
                    href="https://www.openstreetmap.org/copyright"
                    target="_blank"
                    rel="noreferrer"
                >
                    OpenStreetMap
                </a>
                {' · '}
                <a
                    href="https://www.openstreetmap.org/fixthemap"
                    target="_blank"
                    rel="noreferrer"
                >
                    Исправить карту
                </a>
            </div>
        </div>
    );
};
