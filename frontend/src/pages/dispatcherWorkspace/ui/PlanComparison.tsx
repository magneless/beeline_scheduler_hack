import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { RotateCw, X } from 'lucide-react';

import { comparePlans } from 'shared/api';
import { type Plan, type SolveMode } from 'shared/api/types/contracts';
import { workTypeLabel } from 'shared/lib/config';
import { displayEngineer, formatClock, formatKm } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import {
    CalculationProgressBar,
    type CalculationState,
} from 'shared/ui/calculationProgress';
import { MapView } from 'shared/ui/map';

import { routeColor } from '../lib/routeColors';
import { buildMapModel } from '../lib/utils';

type Props = {
    scenarioId: string;
    currentPlanId: string;
    initialMode: SolveMode;
    onClose: () => void;
};

const modeLabel: Record<SolveMode, string> = {
    baseline: 'Базовый',
    optimized: 'Оптимизированный',
};

const Summary = ({
    plan,
    mode,
    active,
    onSelect,
}: {
    plan: Plan;
    mode: SolveMode;
    active: boolean;
    onSelect: () => void;
}) => (
    <button
        type="button"
        aria-pressed={active}
        onClick={onSelect}
        className={[
            'min-w-0 rounded-[10px] border px-4 py-3 text-left transition-colors',
            active
                ? 'border-primary bg-accent text-foreground'
                : 'border-border bg-white text-foreground hover:bg-muted',
        ].join(' ')}
    >
        <strong className="block text-sm">{modeLabel[mode]}</strong>
        <span className="mt-1 block text-xs text-muted-foreground">
            {plan.metrics.assigned_count} назначено ·{' '}
            {plan.metrics.unassigned_count} без бригады ·{' '}
            {formatKm(plan.metrics.total_distance_m)}
        </span>
    </button>
);

export const PlanComparison = ({
    scenarioId,
    currentPlanId,
    initialMode,
    onClose,
}: Props) => {
    const [mode, setMode] = useState<SolveMode>(initialMode);
    const [engineerId, setEngineerId] = useState<string | null>(null);
    const [calculation, setCalculation] = useState<CalculationState | null>(
        null
    );
    const { data, isPending, error, refetch } = useQuery({
        queryKey: ['plan-comparison', scenarioId, currentPlanId],
        queryFn: async ({ signal }) => {
            const startedAt = Date.now();
            setCalculation({ startedAt, lastEventAt: startedAt });
            try {
                return await comparePlans(scenarioId, currentPlanId, {
                    signal,
                    onHeartbeat: () =>
                        setCalculation(
                            (current) =>
                                current && {
                                    ...current,
                                    lastEventAt: Date.now(),
                                }
                        ),
                    onProgress: (progress) =>
                        setCalculation(
                            (current) =>
                                current && {
                                    ...current,
                                    progress,
                                    lastEventAt: Date.now(),
                                }
                        ),
                });
            } finally {
                setCalculation(null);
            }
        },
        retry: false,
    });

    useEffect(() => {
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                onClose();
            }
        };
        window.addEventListener('keydown', closeOnEscape);
        return () => window.removeEventListener('keydown', closeOnEscape);
    }, [onClose]);

    const plan = data?.[mode === 'baseline' ? 'baseline' : 'optimized'];
    const mapModel =
        data && plan
            ? buildMapModel(data.snapshot, plan, engineerId, false, null, true)
            : { markers: [], polylines: [] };
    const route = plan?.routes.find((item) => item.engineer_id === engineerId);
    const orders = new Map(
        data?.snapshot.orders.map((item) => [item.id, item]) ?? []
    );
    const locations = new Map(
        data?.snapshot.locations.map((item) => [item.id, item.address]) ?? []
    );
    const distances = new Map(
        plan?.metrics.per_engineer.map((item) => [
            item.engineer_id,
            item.distance_m,
        ]) ?? []
    );

    return (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/45 p-3">
            <section
                role="dialog"
                aria-modal="true"
                aria-label="Сравнение маршрутов с начала дня"
                className={[
                    'flex h-[min(92vh,900px)] w-full max-w-[1400px] flex-col',
                    'overflow-hidden rounded-[16px] border border-border bg-white shadow-2xl',
                ].join(' ')}
            >
                <header className="flex shrink-0 items-start justify-between gap-4 border-b border-border px-5 py-4">
                    <div>
                        <h2 className="text-lg font-semibold">
                            Сравнение маршрутов
                        </h2>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                            Оба варианта собраны с начала дня на одинаковых
                            данных. Рабочий план не меняется.
                        </p>
                    </div>
                    <Button
                        size="icon"
                        variant="ghost"
                        onClick={onClose}
                        aria-label="Закрыть сравнение"
                    >
                        <X className="size-5" />
                    </Button>
                </header>
                {isPending ? (
                    <div className="flex flex-1 items-center justify-center p-5">
                        {calculation ? (
                            <CalculationProgressBar
                                state={calculation}
                                title="Сравнение маршрутов"
                                className="w-full max-w-[420px]"
                            />
                        ) : (
                            <p
                                role="status"
                                className="text-sm text-muted-foreground"
                            >
                                Начинаем сравнение…
                            </p>
                        )}
                    </div>
                ) : error || !data || !plan ? (
                    <div className="flex flex-1 flex-col items-center justify-center gap-3 p-5 text-center text-sm">
                        <p role="alert">
                            {error instanceof Error
                                ? error.message
                                : 'Не удалось построить варианты'}
                        </p>
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={() => void refetch()}
                        >
                            <RotateCw className="size-4" /> Повторить
                        </Button>
                    </div>
                ) : (
                    <>
                        <div className="grid shrink-0 grid-cols-2 gap-2 border-b border-border p-3">
                            <Summary
                                plan={data.baseline}
                                mode="baseline"
                                active={mode === 'baseline'}
                                onSelect={() => setMode('baseline')}
                            />
                            <Summary
                                plan={data.optimized}
                                mode="optimized"
                                active={mode === 'optimized'}
                                onSelect={() => setMode('optimized')}
                            />
                        </div>
                        <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
                            <div className="relative min-h-[280px] min-w-0 flex-1">
                                <MapView
                                    markers={mapModel.markers}
                                    polylines={mapModel.polylines}
                                    fitToken={`${mode}:${engineerId ?? 'all'}`}
                                    onMarkerClick={(orderId) => {
                                        const assigned = plan.routes.find(
                                            (item) =>
                                                item.visits.some(
                                                    (visit) =>
                                                        visit.order_id ===
                                                        orderId
                                                )
                                        );
                                        if (assigned) {
                                            setEngineerId(assigned.engineer_id);
                                        }
                                    }}
                                />
                            </div>
                            <aside
                                className={[
                                    'flex max-h-[40vh] w-full shrink-0 flex-col overflow-y-auto',
                                    'border-t border-border bg-white lg:max-h-none lg:w-[340px]',
                                    'lg:border-l lg:border-t-0',
                                ].join(' ')}
                            >
                                <div className="sticky top-0 border-b border-border bg-white p-3">
                                    <div className="flex items-center justify-between gap-2">
                                        <strong className="text-sm">
                                            Бригады и путь
                                        </strong>
                                        {engineerId ? (
                                            <button
                                                type="button"
                                                className="text-xs text-primary hover:underline"
                                                onClick={() =>
                                                    setEngineerId(null)
                                                }
                                            >
                                                Все бригады
                                            </button>
                                        ) : null}
                                    </div>
                                </div>
                                <div className="space-y-1 p-2">
                                    {plan.routes.map((item) => (
                                        <button
                                            type="button"
                                            key={item.engineer_id}
                                            onClick={() =>
                                                setEngineerId(
                                                    engineerId ===
                                                        item.engineer_id
                                                        ? null
                                                        : item.engineer_id
                                                )
                                            }
                                            aria-pressed={
                                                engineerId === item.engineer_id
                                            }
                                            className={[
                                                'flex w-full items-center gap-2 rounded-[8px] px-3 py-2',
                                                'text-left text-xs hover:bg-muted',
                                                engineerId === item.engineer_id
                                                    ? 'bg-accent'
                                                    : '',
                                            ].join(' ')}
                                        >
                                            <span
                                                className="size-2.5 shrink-0 rounded-full"
                                                style={{
                                                    background: routeColor(
                                                        item.engineer_id
                                                    ),
                                                }}
                                            />
                                            <span className="min-w-0 flex-1 truncate">
                                                {displayEngineer(
                                                    item.engineer_id
                                                )}
                                            </span>
                                            <span className="text-muted-foreground">
                                                {item.visits.length} ·{' '}
                                                {formatKm(
                                                    distances.get(
                                                        item.engineer_id
                                                    ) ?? 0
                                                )}
                                            </span>
                                        </button>
                                    ))}
                                </div>
                                {route ? (
                                    <ol className="space-y-1 border-t border-border p-3">
                                        {route.visits.map((visit, index) => {
                                            const order = orders.get(
                                                visit.order_id
                                            );
                                            return (
                                                <li
                                                    key={visit.order_id}
                                                    className="rounded-[8px] bg-muted/60 p-2 text-xs"
                                                >
                                                    <strong>
                                                        {index + 1}.{' '}
                                                        {order
                                                            ? workTypeLabel[
                                                                  order
                                                                      .work_type
                                                              ]
                                                            : 'Визит'}{' '}
                                                        ·{' '}
                                                        {formatClock(
                                                            visit.start_at,
                                                            data.snapshot
                                                                .timezone
                                                        )}
                                                    </strong>
                                                    <span className="mt-0.5 block text-muted-foreground">
                                                        {order
                                                            ? locations.get(
                                                                  order.location_id
                                                              )
                                                            : 'Адрес не указан'}
                                                    </span>
                                                </li>
                                            );
                                        })}
                                    </ol>
                                ) : null}
                            </aside>
                        </div>
                    </>
                )}
            </section>
        </div>
    );
};
