import { CalendarRange, PanelRight } from 'lucide-react';

import {
    type Metrics,
    type Run,
    type Snapshot,
} from 'shared/api/types/contracts';
import { regionLabel, runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { cn, formatCount, formatDay, formatKm } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { EventTimeField } from './EventTimeField';
import { RegionSwitcher } from './RegionSwitcher';
import { TopbarMetric } from './TopbarMetric';
import { workspaceCopy } from '../lib/config';
import { useEmergencyControls } from '../model/useEmergencyControls';

type WorkspaceTopbarProps = {
    snapshot: Snapshot | undefined;
    metrics: Metrics | undefined;
    compare: TypeOrNull<Metrics> | undefined;
    compareSource?: TypeOrNull<'baseline' | 'previous'>;
    canEvent: boolean;
    canRebuild: boolean;
    buildPending: boolean;
    eventPending: boolean;
    runStatus?: TypeOrNull<Run['status']>;
    scheduleOpen: boolean;
    panelOpen: boolean;
    timezone: string;
    defaultOccurredAt: string;
    onEmergency: (occurredAt: string) => void;
    onRebuild: () => void;
    onToggleSchedule: () => void;
    onTogglePanel: () => void;
};

const formatCompareHint = (
    before: number | string | undefined,
    now: number | string,
    source: TypeOrNull<'baseline' | 'previous'> | undefined
) => {
    if (before === undefined || before === now) {
        return undefined;
    }

    if (source === 'baseline') {
        return workspaceCopy.wasBaseline(before);
    }

    if (source === 'previous') {
        return workspaceCopy.wasPrevious(before);
    }

    return workspaceCopy.was(before);
};

export const WorkspaceTopbar = ({
    snapshot,
    metrics,
    compare,
    compareSource,
    canEvent,
    canRebuild,
    buildPending,
    eventPending,
    runStatus,
    scheduleOpen,
    panelOpen,
    timezone,
    defaultOccurredAt,
    onEmergency,
    onRebuild,
    onToggleSchedule,
    onTogglePanel,
}: WorkspaceTopbarProps) => {
    const emergency = useEmergencyControls(defaultOccurredAt);

    const regionName = snapshot
        ? (regionLabel[snapshot.region_id] ?? snapshot.region_id)
        : workspaceCopy.shiftFallback;
    const dayLabel = snapshot
        ? `${formatDay(snapshot.date)} · ${formatCount(snapshot.orders.length, [
              'заявка',
              'заявки',
              'заявок',
          ])}`
        : workspaceCopy.loading;
    const metricsFallback =
        runStatus && runStatus !== 'succeeded'
            ? runStatusLabel[runStatus]
            : workspaceCopy.mapHint;

    const handleSubmitEmergency = () => {
        emergency.submit(onEmergency);
    };

    return (
        <div className="pointer-events-none absolute inset-x-4 top-4 z-30 flex items-start gap-2">
            <div
                className={cn(
                    'pointer-events-auto flex max-w-[28%] min-w-0 items-center',
                    'gap-2 rounded-full bg-card px-4 py-2'
                )}
                style={{ boxShadow: 'var(--shadow-soft)' }}
            >
                <span className="shrink-0 text-sm font-bold">{regionName}</span>
                <span className="truncate text-sm text-muted-foreground">
                    {dayLabel}
                </span>
            </div>
            <div
                className={cn(
                    'pointer-events-auto mx-auto flex min-w-0 flex-wrap items-center',
                    'justify-center gap-x-3 gap-y-1 rounded-full bg-card px-4 py-2'
                )}
                style={{ boxShadow: 'var(--shadow-soft)' }}
            >
                {metrics ? (
                    <>
                        <TopbarMetric
                            value={String(metrics.assigned_count)}
                            label={workspaceCopy.metricAssigned}
                            hint={formatCompareHint(
                                compare?.assigned_count,
                                metrics.assigned_count,
                                compareSource
                            )}
                        />
                        <TopbarMetric
                            value={String(metrics.unassigned_count)}
                            label={workspaceCopy.metricUnassigned}
                            hint={formatCompareHint(
                                compare?.unassigned_count,
                                metrics.unassigned_count,
                                compareSource
                            )}
                        />
                        <TopbarMetric
                            value={String(metrics.completed_count)}
                            label={workspaceCopy.metricCompleted}
                            hint={formatCompareHint(
                                compare?.completed_count,
                                metrics.completed_count,
                                compareSource
                            )}
                        />
                        <TopbarMetric
                            value={String(metrics.used_engineer_count)}
                            label={workspaceCopy.metricCrews}
                            hint={formatCompareHint(
                                compare?.used_engineer_count,
                                metrics.used_engineer_count,
                                compareSource
                            )}
                        />
                        <TopbarMetric
                            value={formatKm(metrics.total_distance_m)}
                            label={workspaceCopy.metricDistance}
                            hint={formatCompareHint(
                                compare
                                    ? formatKm(compare.total_distance_m)
                                    : undefined,
                                formatKm(metrics.total_distance_m),
                                compareSource
                            )}
                        />
                    </>
                ) : (
                    <span className="text-sm text-muted-foreground">
                        {metricsFallback}
                    </span>
                )}
            </div>
            <div className="pointer-events-auto relative ml-auto flex shrink-0 items-center gap-2">
                {canRebuild && metrics ? (
                    <Button
                        size="sm"
                        variant="outline"
                        disabled={buildPending || eventPending}
                        onClick={onRebuild}
                    >
                        {buildPending
                            ? workspaceCopy.rebuildPending
                            : workspaceCopy.rebuildPlan}
                    </Button>
                ) : null}
                {canEvent ? (
                    <div className="relative">
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={eventPending}
                            title={workspaceCopy.emergencyTitle}
                            onClick={emergency.toggle}
                        >
                            {eventPending
                                ? workspaceCopy.emergencyPending
                                : workspaceCopy.emergency}
                        </Button>
                        {emergency.open ? (
                            <div
                                className={cn(
                                    'absolute z-40 w-80 max-w-[calc(100vw-2rem)]',
                                    'space-y-2 rounded-[22px] bg-card p-3',
                                    panelOpen
                                        ? 'top-0 right-[calc(100%+16px)]'
                                        : 'top-11 right-0'
                                )}
                                style={{ boxShadow: 'var(--shadow-soft)' }}
                            >
                                <EventTimeField
                                    value={emergency.occurredAt}
                                    timezone={timezone}
                                    onChange={emergency.setOccurredAt}
                                />
                                <Button
                                    size="sm"
                                    className="w-full"
                                    disabled={eventPending}
                                    onClick={handleSubmitEmergency}
                                >
                                    {workspaceCopy.emergencySubmit}
                                </Button>
                            </div>
                        ) : null}
                    </div>
                ) : null}
                <Button
                    size="sm"
                    variant={scheduleOpen ? 'default' : 'outline'}
                    onClick={onToggleSchedule}
                >
                    <CalendarRange className="size-4" />
                    {workspaceCopy.slots}
                </Button>
                <Button
                    size="icon"
                    variant={panelOpen ? 'default' : 'outline'}
                    className="size-9"
                    onClick={onTogglePanel}
                    aria-label={workspaceCopy.panelAria}
                >
                    <PanelRight className="size-4" />
                </Button>
                <RegionSwitcher currentRegionId={snapshot?.region_id} />
            </div>
        </div>
    );
};
