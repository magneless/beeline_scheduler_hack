import { useState } from 'react';
import { CalendarRange, PanelLeft, Plus, RotateCw } from 'lucide-react';

import { type PlanEventInput } from 'features/applyPlanEvent';
import {
    type Metrics,
    type Run,
    type Snapshot,
    type SolveMode,
} from 'shared/api/types/contracts';
import { regionLabel, runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { formatDay, fromClockInput, toClockInput } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { Label } from 'shared/ui/label';
import { TimeSelect } from 'shared/ui/timeSelect';

import { OrderForm } from './OrderForm';
import { RegionSwitcher } from './RegionSwitcher';

type WorkspaceTopbarProps = {
    solveMode: SolveMode;
    savedSolveMode?: SolveMode;
    onSolveMode: (mode: SolveMode) => void;
    snapshot: Snapshot | undefined;
    metrics: Metrics | undefined;
    canEvent: boolean;
    canRebuild: boolean;
    canCompare: boolean;
    buildPending: boolean;
    eventPending: boolean;
    progressVisible: boolean;
    runStatus?: TypeOrNull<Run['status']>;
    scheduleOpen: boolean;
    panelOpen: boolean;
    timezone: string;
    defaultOccurredAt: string;
    onScenarioTime: (time: string) => void;
    hasProposal: boolean;
    onOpenProposal: () => void;
    onNewOrder: (input: PlanEventInput) => void;
    onRebuild: () => void;
    onCompare: () => void;
    onToggleSchedule: () => void;
    onTogglePanel: () => void;
};

export const WorkspaceTopbar = ({
    solveMode,
    savedSolveMode,
    onSolveMode,
    snapshot,
    metrics,
    canEvent,
    canRebuild,
    canCompare,
    buildPending,
    eventPending,
    progressVisible,
    runStatus,
    scheduleOpen,
    panelOpen,
    timezone,
    defaultOccurredAt,
    onScenarioTime,
    hasProposal,
    onOpenProposal,
    onNewOrder,
    onRebuild,
    onCompare,
    onToggleSchedule,
    onTogglePanel,
}: WorkspaceTopbarProps) => {
    const [orderFormOpen, setOrderFormOpen] = useState(false);
    return (
        <header
            className={[
                'relative z-30 flex shrink-0 flex-wrap items-center',
                'justify-between gap-3 border-b border-border bg-white px-5',
                'py-4',
            ].join(' ')}
        >
            <div className="flex min-w-0 items-center gap-3">
                <Button
                    size="icon"
                    variant="ghost"
                    className="size-9 rounded-[8px] text-muted-foreground"
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
                    <h1 className="text-lg leading-tight font-bold tracking-tight text-foreground">
                        Диспетчерская
                    </h1>
                    <p className="mt-1 text-xs text-muted-foreground">
                        {snapshot
                            ? `${regionLabel[snapshot.region_id] ?? snapshot.region_id} · ${formatDay(snapshot.date)}`
                            : 'Загрузка смены…'}
                    </p>
                </div>
                {metrics ? (
                    <span className="ml-3 hidden border-l border-border pl-4 text-xs text-muted-foreground xl:block">
                        <strong className="font-semibold text-foreground">
                            {metrics.assigned_count}
                        </strong>{' '}
                        в плане <span className="mx-2">·</span>
                        <strong className="font-semibold text-foreground">
                            {metrics.completed_count}
                        </strong>{' '}
                        выполнено
                    </span>
                ) : null}
            </div>
            <div className="flex flex-wrap items-center gap-2">
                {(buildPending || eventPending) && !progressVisible ? (
                    <span
                        role="status"
                        className="text-xs text-muted-foreground"
                    >
                        {runStatus
                            ? runStatusLabel[runStatus]
                            : 'Рассчитываем маршруты…'}
                    </span>
                ) : null}
                {snapshot ? (
                    <div className="w-[140px] space-y-1">
                        <Label>Время сценария</Label>
                        <TimeSelect
                            value={toClockInput(defaultOccurredAt, timezone)}
                            aria-label="Время сценария"
                            className="w-full rounded-[6px] border border-border bg-white"
                            onChange={(clock) =>
                                onScenarioTime(
                                    fromClockInput(
                                        snapshot.date,
                                        clock,
                                        timezone
                                    )
                                )
                            }
                        />
                    </div>
                ) : null}
                {hasProposal ? (
                    <Button
                        size="sm"
                        variant="outline"
                        onClick={onOpenProposal}
                    >
                        Варианты плана
                    </Button>
                ) : null}
                <label className="flex items-center gap-2 text-xs text-muted-foreground">
                    Алгоритм
                    <select
                        aria-label="Алгоритм расчёта маршрутов"
                        value={solveMode}
                        onChange={(event) =>
                            onSolveMode(event.target.value as SolveMode)
                        }
                        disabled={
                            !snapshot ||
                            buildPending ||
                            eventPending ||
                            hasProposal
                        }
                        title="Применяется при следующем расчёте маршрутов"
                        className={[
                            'h-9 max-w-[180px] rounded-[8px] border border-border bg-white px-2',
                            'text-xs font-medium text-foreground focus:outline-primary disabled:opacity-50',
                        ].join(' ')}
                    >
                        <option value="optimized">Оптимизированный</option>
                        <option value="baseline">Базовый (по порядку)</option>
                    </select>
                </label>
                {canCompare ? (
                    <Button
                        size="sm"
                        variant="outline"
                        disabled={buildPending || eventPending || hasProposal}
                        onClick={onCompare}
                        className="rounded-[8px]"
                    >
                        Собрать с нуля
                    </Button>
                ) : null}
                {canRebuild && metrics ? (
                    <Button
                        size="sm"
                        variant={
                            savedSolveMode && savedSolveMode !== solveMode
                                ? 'default'
                                : 'outline'
                        }
                        disabled={buildPending || eventPending || hasProposal}
                        onClick={onRebuild}
                        title="Заменить рабочий план новым расчётом до начала событий"
                        className="rounded-[8px]"
                    >
                        <RotateCw className="size-3.5" />
                        {buildPending
                            ? 'Считаем…'
                            : savedSolveMode && savedSolveMode !== solveMode
                              ? 'Применить алгоритм'
                              : 'Пересобрать'}
                    </Button>
                ) : null}
                <Button
                    size="sm"
                    variant="outline"
                    disabled={!metrics}
                    onClick={onToggleSchedule}
                    aria-pressed={scheduleOpen}
                    className="rounded-[8px] border-border"
                >
                    <CalendarRange className="size-4" />
                    {scheduleOpen ? 'Скрыть расписание' : 'Расписание'}
                </Button>
                <RegionSwitcher currentRegionId={snapshot?.region_id} />
                {canEvent ? (
                    <div className="relative">
                        <Button
                            size="sm"
                            disabled={
                                eventPending || buildPending || hasProposal
                            }
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
