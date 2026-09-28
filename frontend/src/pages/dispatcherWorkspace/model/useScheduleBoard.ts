import {
    useCallback,
    useEffect,
    useLayoutEffect,
    useMemo,
    useRef,
    useState,
} from 'react';
import { type DateTime } from 'luxon';

import { type TypeOrNull } from 'shared/lib/types';
import { displayEngineer } from 'shared/lib/utils';

import { scheduleLayout } from '../lib/config';
import {
    buildHourTicks,
    fitZoom,
    isEditableTarget,
    laneIsLive,
    nextZoom,
    ZOOM_DEFAULT,
    ZOOM_MIN,
} from '../lib/utils';

import { type ScheduleLane } from './types';

type LaneFilter = 'all' | 'live';

const {
    pastPad: PAST_PAD,
    visibleRows: VISIBLE_ROWS,
    rowComfort: ROW_COMFORT,
    rowDense: ROW_DENSE,
    hScroll: H_SCROLL,
} = scheduleLayout;

const laneName = (lane: ScheduleLane) =>
    lane.label ?? displayEngineer(lane.engineerId);

type UseScheduleBoardParams = {
    lanes: ScheduleLane[];
    focusAt: DateTime;
    selectedEngineerId: TypeOrNull<string>;
    selectedOrderId?: TypeOrNull<string>;
};

export const useScheduleBoard = ({
    lanes,
    focusAt,
    selectedEngineerId,
    selectedOrderId,
}: UseScheduleBoardParams) => {
    const hoursRef = useRef<HTMLDivElement>(null);
    const peopleRef = useRef<HTMLDivElement>(null);
    const gridRef = useRef<HTMLDivElement>(null);
    const chartRef = useRef<HTMLDivElement>(null);
    const syncLock = useRef(false);
    const primed = useRef(false);
    const pendingLeft = useRef<TypeOrNull<number>>(null);
    const pxRef = useRef(ZOOM_DEFAULT);
    const spaceRef = useRef(false);
    const panRef = useRef<
        TypeOrNull<{
            x: number;
            y: number;
            left: number;
            top: number;
        }>
    >(null);
    const [query, setQuery] = useState('');
    const [laneFilter, setLaneFilter] = useState<LaneFilter>('all');
    const [pxPerHour, setPxPerHour] = useState(ZOOM_DEFAULT);
    const [hand, setHand] = useState(false);
    const [panning, setPanning] = useState(false);
    const [scrollbarHeight, setScrollbarHeight] = useState(0);

    const dense = lanes.length > 6;
    const rowPx = dense ? ROW_DENSE : ROW_COMFORT;
    const start = lanes[0]?.start;
    const end = lanes[0]?.end;
    const hourCount =
        start && end ? Math.max(end.diff(start, 'hours').hours, 1 / 60) : 12;
    const canvasWidth = hourCount * pxPerHour;
    const ticks =
        start && end ? buildHourTicks(start, hourCount, pxPerHour) : [];
    const visibleLanes = useMemo(() => {
        const needle = query.trim().toLowerCase();

        return lanes.filter((lane) => {
            if (needle && !laneName(lane).toLowerCase().includes(needle)) {
                return false;
            }

            if (laneFilter === 'live' && !laneIsLive(lane, focusAt)) {
                return false;
            }

            return true;
        });
    }, [focusAt, laneFilter, lanes, query]);
    const liveCount = useMemo(
        () => lanes.filter((lane) => laneIsLive(lane, focusAt)).length,
        [focusAt, lanes]
    );

    pxRef.current = pxPerHour;

    const syncFromGrid = () => {
        const grid = gridRef.current;

        if (!grid || syncLock.current) {
            return;
        }

        syncLock.current = true;

        if (hoursRef.current) {
            hoursRef.current.scrollLeft = grid.scrollLeft;
        }

        if (peopleRef.current) {
            peopleRef.current.scrollTop = grid.scrollTop;
        }

        syncLock.current = false;
    };

    const syncFromPeople = () => {
        const people = peopleRef.current;
        const grid = gridRef.current;

        if (!people || !grid || syncLock.current) {
            return;
        }

        syncLock.current = true;
        grid.scrollTop = people.scrollTop;
        syncLock.current = false;
    };

    const scrollToFocus = useCallback(
        (smooth = false) => {
            const grid = gridRef.current;

            if (!grid || !start || !end) {
                return;
            }

            const total = end.toMillis() - start.toMillis();
            const x =
                ((focusAt.toMillis() - start.toMillis()) / total) *
                (hourCount * pxRef.current);
            const left = Math.max(
                0,
                Math.min(x - PAST_PAD, grid.scrollWidth - grid.clientWidth)
            );

            if (smooth) {
                grid.scrollTo({ left, behavior: 'smooth' });
                return;
            }

            grid.scrollLeft = left;
            syncFromGrid();
        },
        [end, focusAt, hourCount, start]
    );

    const applyZoom = useCallback(
        (next: number, anchorClientX?: number) => {
            const grid = gridRef.current;
            const current = pxRef.current;

            if (!grid || next === current) {
                return;
            }

            const oldWidth = hourCount * current;
            const playhead =
                start && end
                    ? ((focusAt.toMillis() - start.toMillis()) /
                          (end.toMillis() - start.toMillis())) *
                      oldWidth
                    : grid.scrollLeft + grid.clientWidth / 2;
            const offset =
                anchorClientX === undefined
                    ? playhead - grid.scrollLeft
                    : anchorClientX - grid.getBoundingClientRect().left;
            const anchor = grid.scrollLeft + offset;
            const ratio = oldWidth ? anchor / oldWidth : 0;

            pendingLeft.current = ratio * (hourCount * next) - offset;
            setPxPerHour(next);
        },
        [end, focusAt, hourCount, start]
    );

    const zoomBy = useCallback(
        (direction: 1 | -1, anchorClientX?: number) => {
            const width = gridRef.current?.clientWidth;
            const minimum = width
                ? Math.min(ZOOM_MIN, fitZoom(hourCount, width))
                : ZOOM_MIN;
            applyZoom(
                nextZoom(pxRef.current, direction, minimum),
                anchorClientX
            );
        },
        [applyZoom, hourCount]
    );

    const zoomFit = useCallback(() => {
        const grid = gridRef.current;

        if (!grid) {
            return;
        }

        if (grid.clientWidth === 0) {
            return;
        }
        pendingLeft.current = 0;
        setPxPerHour(fitZoom(hourCount, grid.clientWidth));
    }, [hourCount]);

    const nowLeft =
        start && end
            ? ((focusAt.toMillis() - start.toMillis()) /
                  (end.toMillis() - start.toMillis())) *
              100
            : null;
    const nowVisible =
        start && end && nowLeft !== null && focusAt >= start && focusAt <= end;
    const chartHeight =
        Math.min(Math.max(visibleLanes.length, 1), VISIBLE_ROWS) * rowPx +
        H_SCROLL;
    const zoomLabel = `${Math.round((pxPerHour / ZOOM_DEFAULT) * 100)}%`;

    const handleQueryChange = (value: string) => {
        setQuery(value);
    };

    const handleToggleLive = () => {
        setLaneFilter((current) => (current === 'live' ? 'all' : 'live'));
    };

    const handleZoomOut = () => {
        zoomBy(-1);
    };

    const handleZoomIn = () => {
        zoomBy(1);
    };

    useLayoutEffect(() => {
        const grid = gridRef.current;

        if (!grid || pendingLeft.current === null) {
            return;
        }

        grid.scrollLeft = Math.max(0, pendingLeft.current);
        pendingLeft.current = null;
        syncFromGrid();
    }, [pxPerHour]);

    useLayoutEffect(() => {
        const grid = gridRef.current;
        if (!grid) {
            return;
        }
        const update = () =>
            setScrollbarHeight(
                Math.max(0, grid.offsetHeight - grid.clientHeight)
            );
        const observer = new ResizeObserver(update);
        observer.observe(grid);
        if (grid.firstElementChild) {
            observer.observe(grid.firstElementChild);
        }
        update();
        return () => observer.disconnect();
    }, [visibleLanes.length, pxPerHour]);

    useLayoutEffect(() => {
        if (primed.current || !lanes.length) {
            return;
        }

        primed.current = true;
        scrollToFocus();
    }, [lanes.length, scrollToFocus]);

    useLayoutEffect(() => {
        const grid = gridRef.current;

        if (!grid || !selectedEngineerId) {
            return;
        }

        const index = visibleLanes.findIndex(
            (lane) => lane.engineerId === selectedEngineerId
        );

        if (index < 0) {
            return;
        }

        const top = index * rowPx;
        const view = grid.clientHeight;

        if (top < grid.scrollTop || top + rowPx > grid.scrollTop + view) {
            grid.scrollTop = Math.max(0, top - rowPx);
        }
    }, [rowPx, selectedEngineerId, visibleLanes]);

    useEffect(() => {
        const grid = gridRef.current;
        const laneIndex = visibleLanes.findIndex((lane) =>
            lane.blocks.some(
                (item) =>
                    item.kind === 'work' && item.orderId === selectedOrderId
            )
        );
        const block = visibleLanes[laneIndex]?.blocks.find(
            (item) => item.kind === 'work' && item.orderId === selectedOrderId
        );
        if (!grid || !block || !start) {
            return;
        }
        const top = laneIndex * rowPx;
        if (
            top < grid.scrollTop ||
            top + rowPx > grid.scrollTop + grid.clientHeight
        ) {
            grid.scrollTop = Math.max(0, top - rowPx);
        }
        const left = block.start.diff(start, 'hours').hours * pxRef.current;
        const right = block.end.diff(start, 'hours').hours * pxRef.current;
        if (
            left < grid.scrollLeft ||
            right > grid.scrollLeft + grid.clientWidth
        ) {
            grid.scrollLeft = Math.max(0, left - 24);
        }
        syncFromGrid();
    }, [rowPx, selectedOrderId, start, visibleLanes]);

    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if (
                !chartRef.current?.contains(document.activeElement) ||
                isEditableTarget(event.target)
            ) {
                return;
            }

            if (event.code === 'Space') {
                if ((event.target as HTMLElement).closest('button')) {
                    return;
                }
                event.preventDefault();
                spaceRef.current = true;
                setHand(true);
                return;
            }

            if (event.metaKey || event.ctrlKey || event.altKey) {
                return;
            }

            if (event.code === 'Equal' || event.code === 'NumpadAdd') {
                event.preventDefault();
                zoomBy(1);
                return;
            }

            if (event.code === 'Minus' || event.code === 'NumpadSubtract') {
                event.preventDefault();
                zoomBy(-1);
                return;
            }

            if (event.code === 'Digit0' || event.code === 'KeyF') {
                event.preventDefault();
                zoomFit();
                return;
            }

            if (event.code === 'Digit1') {
                event.preventDefault();
                applyZoom(ZOOM_DEFAULT);
                return;
            }

            if (event.code === 'Home' || event.code === 'KeyN') {
                event.preventDefault();
                scrollToFocus(true);
                return;
            }

            const grid = gridRef.current;
            if (!grid) {
                return;
            }
            const vertical =
                event.code === 'ArrowDown'
                    ? rowPx
                    : event.code === 'ArrowUp'
                      ? -rowPx
                      : event.code === 'PageDown'
                        ? grid.clientHeight
                        : event.code === 'PageUp'
                          ? -grid.clientHeight
                          : 0;
            const horizontal =
                event.code === 'ArrowRight'
                    ? H_SCROLL * 4
                    : event.code === 'ArrowLeft'
                      ? -H_SCROLL * 4
                      : 0;
            if (vertical || horizontal) {
                event.preventDefault();
                grid.scrollBy({ left: horizontal, top: vertical });
                syncFromGrid();
            }
        };
        const onKeyUp = (event: KeyboardEvent) => {
            if (event.code !== 'Space') {
                return;
            }

            spaceRef.current = false;
            panRef.current = null;
            setHand(false);
            setPanning(false);
        };

        window.addEventListener('keydown', onKeyDown);
        window.addEventListener('keyup', onKeyUp);

        return () => {
            window.removeEventListener('keydown', onKeyDown);
            window.removeEventListener('keyup', onKeyUp);
        };
    }, [applyZoom, rowPx, scrollToFocus, zoomBy, zoomFit]);

    useEffect(() => {
        const grid = gridRef.current;
        const chart = chartRef.current;

        if (!grid || !chart) {
            return;
        }

        const onWheel = (event: WheelEvent) => {
            if (!event.ctrlKey && !event.metaKey && !event.altKey) {
                if (
                    !grid.contains(event.target as Node) &&
                    (event.shiftKey ||
                        Math.abs(event.deltaX) > Math.abs(event.deltaY))
                ) {
                    event.preventDefault();
                    grid.scrollLeft += event.deltaX || event.deltaY;
                    syncFromGrid();
                }
                return;
            }

            event.preventDefault();
            zoomBy(event.deltaY < 0 ? 1 : -1, event.clientX);
        };
        const onPointerDown = (event: PointerEvent) => {
            if (!spaceRef.current && event.button !== 1) {
                return;
            }

            event.preventDefault();
            panRef.current = {
                x: event.clientX,
                y: event.clientY,
                left: grid.scrollLeft,
                top: grid.scrollTop,
            };
            setPanning(true);
            grid.setPointerCapture(event.pointerId);
        };
        const onPointerMove = (event: PointerEvent) => {
            const pan = panRef.current;

            if (!pan) {
                return;
            }

            grid.scrollLeft = pan.left - (event.clientX - pan.x);
            grid.scrollTop = pan.top - (event.clientY - pan.y);
            syncFromGrid();
        };
        const onPointerUp = () => {
            panRef.current = null;
            setPanning(false);
        };

        chart.addEventListener('wheel', onWheel, { passive: false });
        grid.addEventListener('pointerdown', onPointerDown);
        grid.addEventListener('pointermove', onPointerMove);
        grid.addEventListener('pointerup', onPointerUp);
        grid.addEventListener('pointercancel', onPointerUp);

        return () => {
            chart.removeEventListener('wheel', onWheel);
            grid.removeEventListener('pointerdown', onPointerDown);
            grid.removeEventListener('pointermove', onPointerMove);
            grid.removeEventListener('pointerup', onPointerUp);
            grid.removeEventListener('pointercancel', onPointerUp);
        };
    }, [visibleLanes.length, zoomBy]);

    return {
        hoursRef,
        peopleRef,
        gridRef,
        chartRef,
        query,
        dense,
        rowPx,
        start,
        end,
        canvasWidth,
        ticks,
        visibleLanes,
        liveCount,
        hand,
        panning,
        nowLeft,
        nowVisible,
        chartHeight,
        scrollbarHeight,
        zoomLabel,
        laneFilter,
        pxPerHour,
        syncFromPeople,
        syncFromGrid,
        zoomFit,
        laneName,
        handleQueryChange,
        handleToggleLive,
        handleZoomOut,
        handleZoomIn,
    };
};
