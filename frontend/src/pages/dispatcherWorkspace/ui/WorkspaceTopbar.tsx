import { useState } from 'react';
import { CalendarRange, PanelLeft, Plus, RotateCw } from 'lucide-react';

import { type PlanEventInput } from 'features/applyPlanEvent';
import {
    type Metrics,
    type Run,
    type Snapshot,
} from 'shared/api/types/contracts';
import { regionLabel, runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { formatDay } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { OrderForm } from './OrderForm';
import { RegionSwitcher } from './RegionSwitcher';

type WorkspaceTopbarProps = {
    snapshot: Snapshot | undefined;
    metrics: Metrics | undefined;
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

export const WorkspaceTopbar = ({
    snapshot,
    metrics,
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
    return (
        <header
            className={[
                'relative z-30 flex shrink-0 flex-wrap items-center',
                'justify-between gap-3 border-b border-slate-200 bg-white px-5',
                'py-4',
            ].join(' ')}
        >
            <div className="flex min-w-0 items-center gap-3">
                <Button
                    size="icon"
                    variant="ghost"
                    className="size-9 rounded-[8px] text-slate-500"
                    onClick={onTogglePanel}
                    aria-label={
                        panelOpen
                            ? 'Скрыть рабочую панель'
                            : 'Показать рабочую панель'
                    }
                    aria-expanded={panelOpen}
                >
                    <PanelLeft className="size-5" />
                </Button>
                <div>
                    <h1 className="text-lg leading-tight font-bold tracking-tight text-slate-900">
                        Диспетчерская
                    </h1>
                    <p className="mt-1 text-xs text-slate-500">
                        {snapshot
                            ? `${regionLabel[snapshot.region_id] ?? snapshot.region_id} · ${formatDay(snapshot.date)}`
                            : 'Загрузка смены…'}
                    </p>
                </div>
                {metrics ? (
                    <span className="ml-3 hidden border-l border-slate-200 pl-4 text-xs text-slate-500 xl:block">
                        <strong className="font-semibold text-slate-800">
                            {metrics.assigned_count}
                        </strong>{' '}
                        в плане <span className="mx-2">·</span>
                        <strong className="font-semibold text-slate-800">
                            {metrics.completed_count}
                        </strong>{' '}
                        выполнено
                    </span>
                ) : null}
            </div>
            <div className="flex flex-wrap items-center gap-2">
                {(buildPending || eventPending) && runStatus ? (
                    <span role="status" className="text-xs text-slate-500">
                        {runStatusLabel[runStatus]}
                    </span>
                ) : null}
                {canRebuild && metrics ? (
                    <Button
                        size="sm"
                        variant="ghost"
                        disabled={buildPending || eventPending}
                        onClick={onRebuild}
                        className="rounded-[8px] text-slate-600"
                    >
                        <RotateCw className="size-3.5" />
                        {buildPending ? 'Считаем…' : 'Пересобрать'}
                    </Button>
                ) : null}
                <Button
                    size="sm"
                    variant="outline"
                    disabled={!metrics}
                    onClick={onToggleSchedule}
                    aria-pressed={scheduleOpen}
                    className="rounded-[8px] border-slate-200"
                >
                    <CalendarRange className="size-4" />
                    {scheduleOpen ? 'Скрыть расписание' : 'Расписание'}
                </Button>
                <RegionSwitcher currentRegionId={snapshot?.region_id} />
                {canEvent ? (
                    <div className="relative">
                        <Button
                            size="sm"
                            disabled={eventPending || buildPending}
                            onClick={() => setOrderFormOpen((value) => !value)}
                            aria-expanded={orderFormOpen}
                            className="rounded-[8px]"
                        >
                            <Plus className="size-4" />
                            Новая заявка
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
            </div>
        </header>
    );
};
