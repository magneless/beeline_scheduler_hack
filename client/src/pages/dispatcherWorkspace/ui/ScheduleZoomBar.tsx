import { Maximize2, Minus, Plus } from 'lucide-react';

import styles from './ScheduleBoard.module.scss';

type ScheduleZoomBarProps = {
    zoomLabel: string;
    onZoomOut: () => void;
    onZoomIn: () => void;
    onZoomFit: () => void;
};

export const ScheduleZoomBar = ({
    zoomLabel,
    onZoomOut,
    onZoomIn,
    onZoomFit,
}: ScheduleZoomBarProps) => (
    <div className={styles.zoom}>
        <button
            type="button"
            className={styles.zoomBtn}
            aria-label="Отдалить"
            onClick={onZoomOut}
        >
            <Minus />
        </button>
        <span className={styles.zoomRead}>{zoomLabel}</span>
        <button
            type="button"
            className={styles.zoomBtn}
            aria-label="Приблизить"
            onClick={onZoomIn}
        >
            <Plus />
        </button>
        <button
            type="button"
            className={styles.zoomBtn}
            aria-label="Весь день"
            onClick={onZoomFit}
        >
            <Maximize2 />
        </button>
    </div>
);
