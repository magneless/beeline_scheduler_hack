import { type ReactNode, useEffect, useId, useRef, useState } from 'react';

type Props = {
    map: ReactNode;
    schedule?: ReactNode;
    defaultRatio?: number;
    mapLabel?: string;
};

export const MapScheduleSplit = ({
    map,
    schedule,
    defaultRatio = 0.48,
    mapLabel = 'Карта маршрутов',
}: Props) => {
    const container = useRef<HTMLDivElement>(null);
    const drag = useRef<{ y: number; ratio: number; height: number } | null>(
        null
    );
    const scheduleId = useId();
    const [height, setHeight] = useState(0);
    const [ratio, setRatio] = useState(defaultRatio);
    const [dragging, setDragging] = useState(false);
    const available = Math.max(0, height - 14);
    const min = available ? Math.min(180 / available, 0.5) : 0.2;
    const max = available ? 1 - Math.min(140 / available, 0.5) : 0.8;
    const clamp = (value: number) => Math.max(min, Math.min(max, value));
    const effectiveRatio = clamp(ratio);

    useEffect(() => {
        const element = container.current;
        if (!element) {
            return;
        }
        const observer = new ResizeObserver(([entry]) => {
            setHeight(entry.contentRect.height);
        });
        observer.observe(element);
        return () => observer.disconnect();
    }, []);

    const finishDrag = () => {
        drag.current = null;
        setDragging(false);
    };

    return (
        <div
            ref={container}
            className="grid min-h-0 min-w-0 flex-1 overflow-hidden"
            style={{
                gridTemplateRows: schedule
                    ? `minmax(0, ${1 - effectiveRatio}fr) 14px minmax(0, ${effectiveRatio}fr)`
                    : 'minmax(0, 1fr)',
            }}
        >
            <div className="relative min-h-0 min-w-0" aria-label={mapLabel}>
                {map}
            </div>
            {schedule ? (
                <div
                    role="separator"
                    tabIndex={0}
                    aria-label="Размер расписания"
                    aria-orientation="horizontal"
                    aria-controls={scheduleId}
                    aria-valuemin={Math.round(min * 100)}
                    aria-valuemax={Math.round(max * 100)}
                    aria-valuenow={Math.round(effectiveRatio * 100)}
                    aria-valuetext={`Расписание: ${Math.round(effectiveRatio * 100)}%`}
                    title="Потяните, чтобы изменить высоту расписания. Двойной щелчок — исходный размер."
                    className={[
                        'group z-10 flex touch-none cursor-row-resize items-center justify-center',
                        'border-y border-border outline-none select-none hover:bg-accent focus-visible:bg-accent',
                        dragging ? 'bg-accent' : 'bg-white',
                    ].join(' ')}
                    onPointerDown={(event) => {
                        if (event.button !== 0 || available === 0) {
                            return;
                        }
                        event.preventDefault();
                        event.currentTarget.focus();
                        event.currentTarget.setPointerCapture(event.pointerId);
                        drag.current = {
                            y: event.clientY,
                            ratio: effectiveRatio,
                            height: available,
                        };
                        setDragging(true);
                    }}
                    onPointerMove={(event) => {
                        const start = drag.current;
                        if (start) {
                            setRatio(
                                clamp(
                                    start.ratio +
                                        (start.y - event.clientY) / start.height
                                )
                            );
                        }
                    }}
                    onPointerUp={(event) => {
                        finishDrag();
                        if (
                            event.currentTarget.hasPointerCapture(
                                event.pointerId
                            )
                        ) {
                            event.currentTarget.releasePointerCapture(
                                event.pointerId
                            );
                        }
                    }}
                    onPointerCancel={finishDrag}
                    onLostPointerCapture={finishDrag}
                    onDoubleClick={() => setRatio(defaultRatio)}
                    onKeyDown={(event) => {
                        const step = event.shiftKey ? 0.1 : 0.02;
                        const positions: Record<string, number> = {
                            ArrowUp: effectiveRatio + step,
                            ArrowDown: effectiveRatio - step,
                            Home: min,
                            End: max,
                        };
                        const next = positions[event.key];
                        if (next !== undefined) {
                            event.preventDefault();
                            setRatio(clamp(next));
                        }
                    }}
                >
                    <span className="h-1 w-12 rounded-full bg-muted-foreground/35 group-hover:bg-primary" />
                </div>
            ) : null}
            {schedule ? (
                <div
                    id={scheduleId}
                    className="min-h-0 min-w-0 overflow-hidden"
                >
                    {schedule}
                </div>
            ) : null}
        </div>
    );
};
