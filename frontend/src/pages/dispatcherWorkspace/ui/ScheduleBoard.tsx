import { type DateTime } from 'luxon';

import { type TypeOrNull } from 'shared/lib/types';
import { cn, formatCount } from 'shared/lib/utils';

import { ScheduleBlockButton } from './ScheduleBlockButton';
import { ScheduleBoardHead } from './ScheduleBoardHead';
import { ScheduleLanePerson } from './ScheduleLanePerson';
import { ScheduleZoomBar } from './ScheduleZoomBar';
import { workspaceCopy } from '../lib/config';
import { routeColor } from '../lib/routeColors';
import { useScheduleBoard } from '../model/useScheduleBoard';

import { type ScheduleLane } from '../model/types';

import styles from './ScheduleBoard.module.scss';

type ScheduleBoardProps = {
    itinerary?: {
        stops: Record<string, { sequence: number; description: string }>;
        summary: string;
        departure: string;
    };
    lanes: ScheduleLane[];
    focusAt: DateTime;
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    onSelectOrder: (id: string) => void;
    onSelectEngineer: (id: TypeOrNull<string>) => void;
    fillHeight?: boolean;
    highlightOrderIds?: string[];
};

export const ScheduleBoard = ({
    itinerary,
    lanes,
    focusAt,
    selectedOrderId,
    selectedEngineerId,
    onSelectOrder,
    onSelectEngineer,
    fillHeight = false,
    highlightOrderIds,
}: ScheduleBoardProps) => {
    const board = useScheduleBoard({
        lanes,
        focusAt,
        selectedEngineerId,
        selectedOrderId,
    });

    return (
        <div
            className={cn(
                styles.board,
                itinerary ? styles.itinerary : '',
                board.dense ? styles.dense : '',
                board.panning ? styles.panning : '',
                board.hand && !board.panning ? styles.ready : '',
                fillHeight ? styles.boardFill : ''
            )}
        >
            <ScheduleBoardHead
                title={itinerary ? 'Порядок визитов' : undefined}
                summary={itinerary?.summary}
                dense={board.dense}
                query={board.query}
                liveFilter={board.laneFilter === 'live'}
                onQueryChange={board.handleQueryChange}
                onToggleLive={board.handleToggleLive}
            />
            {board.visibleLanes.length && board.start && board.end ? (
                <div
                    ref={board.chartRef}
                    tabIndex={0}
                    aria-label="Временная шкала"
                    className={styles.chart}
                    style={{
                        ['--row' as string]: `${board.rowPx}px`,
                        ['--hours' as string]: '28px',
                        height: fillHeight
                            ? undefined
                            : `calc(var(--hours) + ${board.chartHeight}px)`,
                    }}
                >
                    <div className={styles.caption}>
                        {itinerary
                            ? itinerary.departure
                            : formatCount(lanes.length, [
                                  'бригада',
                                  'бригады',
                                  'бригад',
                              ])}
                        {!itinerary && board.liveCount
                            ? workspaceCopy.scheduleOnLine(board.liveCount)
                            : ''}
                    </div>
                    <div ref={board.hoursRef} className={styles.hoursRail}>
                        <div
                            className={styles.hours}
                            style={{ width: board.canvasWidth }}
                        >
                            {board.ticks.map((tick, index) => {
                                const shift =
                                    index === 0
                                        ? '0'
                                        : index === board.ticks.length - 1
                                          ? '-100%'
                                          : '-50%';

                                return (
                                    <span
                                        key={tick.at.toISO() ?? index}
                                        className={styles.hour}
                                        style={{
                                            left: tick.left,
                                            transform: `translate(${shift}, -50%)`,
                                        }}
                                    >
                                        {tick.at.toFormat('HH:mm')}
                                    </span>
                                );
                            })}
                        </div>
                    </div>
                    <div
                        ref={board.peopleRef}
                        className={styles.people}
                        style={{ paddingBottom: board.scrollbarHeight }}
                        onScroll={board.syncFromPeople}
                    >
                        {board.visibleLanes.map((lane) =>
                            itinerary ? (
                                <div
                                    key={lane.engineerId}
                                    className={styles.person}
                                >
                                    <span
                                        className="ml-2 size-2 shrink-0 rounded-full"
                                        style={{
                                            background: routeColor(
                                                lane.engineerId
                                            ),
                                        }}
                                    />
                                    <span className={styles.personName}>
                                        {board.laneName(lane)}
                                    </span>
                                </div>
                            ) : (
                                <ScheduleLanePerson
                                    key={lane.engineerId}
                                    lane={lane}
                                    name={board.laneName(lane)}
                                    dense={board.dense}
                                    active={
                                        selectedEngineerId === lane.engineerId
                                    }
                                    focusAt={focusAt}
                                    onSelect={() =>
                                        onSelectEngineer(lane.engineerId)
                                    }
                                />
                            )
                        )}
                    </div>
                    <div
                        ref={board.gridRef}
                        className={styles.grid}
                        onScroll={board.syncFromGrid}
                    >
                        <div
                            className={styles.canvas}
                            style={{
                                width: board.canvasWidth,
                                ['--hour' as string]: `${board.pxPerHour}px`,
                            }}
                        >
                            {board.nowVisible ? (
                                <>
                                    <div
                                        className={styles.pastVeil}
                                        style={{
                                            width: `${board.nowLeft}%`,
                                        }}
                                    />
                                    <div
                                        className={styles.now}
                                        style={{
                                            left: `${board.nowLeft}%`,
                                        }}
                                    />
                                </>
                            ) : null}
                            {board.visibleLanes.map((lane) => (
                                <div
                                    key={lane.engineerId}
                                    className={cn(
                                        styles.track,
                                        selectedEngineerId === lane.engineerId
                                            ? styles.trackActive
                                            : ''
                                    )}
                                >
                                    {lane.blocks.map((block) => (
                                        <ScheduleBlockButton
                                            key={block.id}
                                            stop={
                                                block.orderId
                                                    ? itinerary?.stops[
                                                          block.orderId
                                                      ]
                                                    : undefined
                                            }
                                            lane={lane}
                                            block={block}
                                            focusAt={focusAt}
                                            selectedOrderId={selectedOrderId}
                                            highlighted={
                                                block.kind === 'work' &&
                                                Boolean(
                                                    block.orderId &&
                                                    highlightOrderIds?.includes(
                                                        block.orderId
                                                    )
                                                )
                                            }
                                            onSelectOrder={onSelectOrder}
                                        />
                                    ))}
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            ) : lanes.length ? (
                <div className={styles.empty}>
                    {workspaceCopy.scheduleEmptyFilter}
                </div>
            ) : (
                <div className={styles.empty}>
                    {workspaceCopy.scheduleEmpty}
                </div>
            )}
            {itinerary &&
            selectedOrderId &&
            itinerary.stops[selectedOrderId] ? (
                <p
                    className={styles.visitDetail}
                    title={itinerary.stops[selectedOrderId].description}
                >
                    {itinerary.stops[selectedOrderId].description}
                </p>
            ) : null}
            <ScheduleZoomBar
                zoomLabel={board.zoomLabel}
                onZoomOut={board.handleZoomOut}
                onZoomIn={board.handleZoomIn}
                onZoomFit={board.zoomFit}
            />
        </div>
    );
};
