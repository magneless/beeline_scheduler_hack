import { useState } from 'react';
import { CalendarRange, PanelRight } from 'lucide-react';

import { type PlanEventInput } from 'features/applyPlanEvent';
import {
    type Metrics,
    type Run,
    type Snapshot,
} from 'shared/api/types/contracts';
import { regionLabel, runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { cn, formatCount, formatDay, formatKm } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { OrderForm } from './OrderForm';
import { RegionSwitcher } from './RegionSwitcher';
import { TopbarMetric } from './TopbarMetric';
import { workspaceCopy } from '../lib/config';

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
    onNewOrder: (input: PlanEventInput) => void;
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
    onNewOrder,
    onRebuild,
    onToggleSchedule,
    onTogglePanel,
}: WorkspaceTopbarProps) => {
    const [orderFormOpen, setOrderFormOpen] = useState(false);

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
                            disabled={eventPending || buildPending}
                            title="Добавить обычную или аварийную заявку"
                            onClick={() => setOrderFormOpen((v) => !v)}
                        >
                            {eventPending
                                ? workspaceCopy.emergencyPending
                                : 'Новая заявка'}
                        </Button>
                        {orderFormOpen && snapshot ? (
                            <OrderForm
                                snapshot={snapshot}
                                timezone={timezone}
                                occurredAt={defaultOccurredAt}
                                pending={eventPending}
                                onSubmit={(input) => {
                                    onNewOrder(input);
                                    setOrderFormOpen(false);
                                }}
                                onClose={() => setOrderFormOpen(false)}
                            />
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
