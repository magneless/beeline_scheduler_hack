import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import { DateTime } from 'luxon';

import {
    type Plan,
    type PlanOption,
    type PlanProposal,
    type Snapshot,
} from 'shared/api';
import {
    changeReasonLabel,
    reasonCodeLabel,
    workTypeLabel,
} from 'shared/lib/config';
import {
    displayEngineer,
    formatClock,
    formatCount,
    formatKm,
} from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { MapView } from 'shared/ui/map';

import { MapScheduleSplit } from './MapScheduleSplit';
import { WorkspaceScheduleDock } from './WorkspaceScheduleDock';
import { WorkTypeBadge } from './WorkTypeBadge';
import { routeColor } from '../lib/routeColors';
import { buildMapModel } from '../lib/utils';
import { buildWorkspaceView } from '../lib/viewModel';

type Props = {
    proposal: PlanProposal;
    unlocatedOrders?: Snapshot['unlocated_orders'];
    accepting: boolean;
    onAccept: (key: string) => void;
    onClose: () => void;
};

const elapsed = (seconds: number) => {
    const minutes = Math.ceil(seconds / 60);
    return minutes >= 60
        ? `${Math.floor(minutes / 60)} ч ${minutes % 60} мин`
        : `${minutes} мин`;
};

const assignmentMap = (routes: Plan['routes']) =>
    new Map(
        routes.flatMap((route) =>
            route.visits.map(
                (visit) =>
                    [
                        visit.order_id,
                        { engineerId: route.engineer_id, visit },
                    ] as const
            )
        )
    );

const assignmentSummary = (
    engineerId: string,
    startAt: string,
    timezone: string
) =>
    `${displayEngineer(engineerId)} · начало ${formatClock(startAt, timezone)}`;

const OptionCard = ({
    option,
    selected,
    onSelect,
}: {
    option: PlanOption;
    selected: boolean;
    onSelect: () => void;
}) => {
    const metrics = option.result.draft.metrics;
    const unresolvedEmergency = option.result.draft.unassigned.some((item) =>
        option.result.target_snapshot.orders.some(
            (order) =>
                order.id === item.order_id &&
                order.work_type === 'emergency' &&
                order.priority === 'urgent'
        )
    );
    return (
        <button
            type="button"
            aria-pressed={selected}
            onClick={onSelect}
            className={[
                'min-w-[185px] flex-1 rounded-[10px] border px-3 py-3 text-left',
                selected
                    ? 'border-primary bg-accent'
                    : 'border-border bg-white hover:bg-muted',
            ].join(' ')}
        >
            <strong className="block text-sm leading-tight">
                {option.label}
            </strong>
            <span className="mt-1 block text-xs text-muted-foreground">
                {metrics.assigned_count} назначено · {metrics.unassigned_count}{' '}
                без бригады
            </span>
            <span className="mt-1 block text-xs text-muted-foreground">
                {option.result.target_snapshot.unlocated_orders?.length
                    ? `${option.result.target_snapshot.unlocated_orders.length} без координат · `
                    : ''}
                {metrics.used_engineer_count} бригад ·{' '}
                {formatKm(metrics.total_distance_m)}
            </span>
            {option.identical_to ? (
                <span className="mt-1 block text-xs font-medium">
                    {option.key === 'late_emergency'
                        ? unresolvedEmergency
                            ? 'Допуск опоздания не помог: другие ограничения назначения сохраняются.'
                            : 'Нет аварий, для которых требуется допуск опоздания. План совпадает.'
                        : 'Совпадает с другим вариантом'}
                </span>
            ) : null}
        </button>
    );
};

export const PlanProposalDialog = ({
    proposal,
    unlocatedOrders,
    accepting,
    onAccept,
    onClose,
}: Props) => {
    const [optionKey, setOptionKey] = useState(
        proposal.options[0]?.key ?? 'strict'
    );
    const [engineerId, setEngineerId] = useState<string | null>(null);
    const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null);
    const option =
        proposal.options.find((item) => item.key === optionKey) ??
        proposal.options[0];
    const rawSnapshot = option?.result.target_snapshot;
    const snapshot = rawSnapshot
        ? {
              ...rawSnapshot,
              unlocated_orders: rawSnapshot.unlocated_orders ?? unlocatedOrders,
          }
        : undefined;
    const plan: Plan | undefined = option
        ? { id: `preview-${proposal.id}`, ...option.result.draft }
        : undefined;
    const mapModel =
        snapshot && plan
            ? buildMapModel(
                  snapshot,
                  plan,
                  engineerId,
                  false,
                  selectedOrderId,
                  true
              )
            : { markers: [], polylines: [] };
    const view = buildWorkspaceView({
        snapshot,
        plan,
        compareMetrics: null,
        selectedOrderId,
        selectedEngineerId: engineerId,
    });
    const earliestVisit = plan?.routes
        .flatMap((route) => route.visits.map((visit) => visit.start_at))
        .sort()[0];
    const focusAt = DateTime.fromISO(
        proposal.event?.occurred_at ?? earliestVisit ?? plan?.as_of ?? '',
        { setZone: true }
    ).setZone(snapshot?.timezone ?? 'Europe/Moscow');
    const orders = new Map(
        snapshot?.orders.map((item) => [item.id, item]) ?? []
    );
    const locations = new Map(
        snapshot?.locations.map((item) => [item.id, item.address]) ?? []
    );
    const reserve = new Set(option?.reserve_engineer_ids ?? []);
    const strictOption = proposal.options.find((item) => item.key === 'strict');
    const strictAssignments = assignmentMap(
        strictOption?.result.draft.routes ?? []
    );
    const assignments = assignmentMap(plan?.routes ?? []);
    const latenessByOrder = new Map(
        option?.lateness.map((item) => [item.order_id, item]) ?? []
    );
    const emergencyHighlights = (snapshot?.orders ?? []).filter((order) => {
        if (
            order.work_type !== 'emergency' ||
            order.status === 'cancelled' ||
            order.status === 'completed'
        ) {
            return false;
        }
        const strict = strictAssignments.get(order.id);
        const current = assignments.get(order.id);
        const changedFromStrict =
            strict?.engineerId !== current?.engineerId ||
            strict?.visit.start_at !== current?.visit.start_at;
        const change = plan?.changes.find((item) => item.order_id === order.id);
        const changedFromCurrent =
            proposal.event &&
            change &&
            (change.before?.engineer_id !== change.after?.engineer_id ||
                change.before?.start_at !== change.after?.start_at);
        return (
            latenessByOrder.has(order.id) ||
            changedFromStrict ||
            changedFromCurrent
        );
    });
    const emergencyHighlightIds = emergencyHighlights.map((order) => order.id);
    const selectOrder = (orderId: string) => {
        setSelectedOrderId(orderId);
        setEngineerId(assignments.get(orderId)?.engineerId ?? null);
    };
    const originalMetrics = proposal.options.find(
        (item) => item.key === 'original'
    )?.result.draft.metrics;
    const originalHint = originalMetrics
        ? `${originalMetrics.assigned_count} в плане · ${originalMetrics.unassigned_count} без бригады`
        : '';

    useEffect(() => {
        const onEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                onClose();
            }
        };
        window.addEventListener('keydown', onEscape);
        return () => window.removeEventListener('keydown', onEscape);
    }, [onClose]);

    if (!option || !snapshot || !plan) {
        return null;
    }
    const currentOrder = selectedOrderId
        ? orders.get(selectedOrderId)
        : undefined;
    return (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/45 p-2 sm:p-3">
            <section
                role="dialog"
                aria-modal="true"
                aria-label="Выбор рабочего плана"
                className={[
                    'flex h-full w-full max-w-[1500px] flex-col overflow-hidden',
                    'rounded-[16px] border border-border bg-white shadow-2xl',
                ].join(' ')}
            >
                <header
                    className={[
                        'flex shrink-0 items-start justify-between gap-3 border-b border-border',
                        'px-4 py-3 sm:px-5',
                    ].join(' ')}
                >
                    <div>
                        <h2 className="text-lg font-semibold">
                            Выберите план для бригад
                        </h2>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                            {proposal.event
                                ? 'Изменения вступят в силу после принятия варианта.'
                                : 'Маршруты начнут действовать после принятия варианта.'}
                        </p>
                        {snapshot.unlocated_orders?.length ? (
                            <p className="mt-1 text-xs text-destructive">
                                Без координат:{' '}
                                {snapshot.unlocated_orders.length}, из них
                                аварий:{' '}
                                {
                                    snapshot.unlocated_orders.filter(
                                        (item) =>
                                            item.order.work_type === 'emergency'
                                    ).length
                                }
                                . Причины — в списке справа.
                            </p>
                        ) : null}
                    </div>
                    <Button
                        size="icon"
                        variant="ghost"
                        onClick={onClose}
                        aria-label="Закрыть варианты"
                    >
                        <X className="size-5" />
                    </Button>
                </header>
                <div className="flex shrink-0 gap-2 overflow-x-auto border-b border-border p-3">
                    {proposal.options
                        .filter((item) => item.key !== 'original')
                        .map((item) => (
                            <OptionCard
                                key={item.key}
                                option={item}
                                selected={item.key === option.key}
                                onSelect={() => {
                                    setOptionKey(item.key);
                                    setEngineerId(null);
                                    setSelectedOrderId(null);
                                }}
                            />
                        ))}
                </div>
                {proposal.options.some((item) => item.key === 'original') ? (
                    <div className="shrink-0 border-b border-border px-4 py-2">
                        <button
                            type="button"
                            aria-pressed={option.key === 'original'}
                            onClick={() => {
                                setOptionKey('original');
                                setEngineerId(null);
                                setSelectedOrderId(null);
                            }}
                            className={[
                                'rounded-[8px] border px-3 py-2 text-left text-xs',
                                option.key === 'original'
                                    ? 'border-primary bg-accent'
                                    : 'border-border hover:bg-muted',
                            ].join(' ')}
                        >
                            <strong>Сохранить прежнее расписание</strong>
                            <span className="ml-2 text-muted-foreground">
                                {originalHint}
                            </span>
                        </button>
                    </div>
                ) : null}
                <div className="flex min-h-0 flex-1 flex-col overflow-y-auto lg:flex-row lg:overflow-hidden">
                    <div className="flex min-h-[360px] min-w-0 flex-1 lg:min-h-0">
                        <MapScheduleSplit
                            map={
                                <MapView
                                    markers={mapModel.markers}
                                    polylines={mapModel.polylines}
                                    selectedId={selectedOrderId}
                                    fitToken={`${proposal.id}:${option.key}:${engineerId ?? 'all'}`}
                                    onMarkerClick={(id) =>
                                        setSelectedOrderId(
                                            id === 'office' ? null : id
                                        )
                                    }
                                />
                            }
                            schedule={
                                <WorkspaceScheduleDock
                                    key={`${proposal.id}-${option.key}`}
                                    fillHeight
                                    highlightOrderIds={emergencyHighlightIds}
                                    lanes={view.lanes}
                                    focusAt={
                                        focusAt.isValid ? focusAt : view.focusAt
                                    }
                                    selectedOrderId={selectedOrderId}
                                    selectedEngineerId={engineerId}
                                    onSelectOrder={setSelectedOrderId}
                                    onSelectEngineer={setEngineerId}
                                />
                            }
                        />
                    </div>
                    <aside
                        className={[
                            'flex max-h-[34vh] w-full shrink-0 flex-col overflow-hidden',
                            'border-t border-border bg-white',
                            'lg:max-h-none lg:w-[350px] lg:border-t-0 lg:border-l',
                        ].join(' ')}
                    >
                        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 text-xs">
                            {currentOrder ? (
                                <div className="rounded-[8px] border border-primary bg-accent p-3">
                                    <WorkTypeBadge
                                        type={currentOrder.work_type}
                                    />
                                    <p className="mt-1">
                                        {locations.get(
                                            currentOrder.location_id
                                        )}
                                    </p>
                                    <p className="mt-1 text-muted-foreground">
                                        Окно{' '}
                                        {formatClock(
                                            currentOrder.window.start,
                                            snapshot.timezone
                                        )}
                                        –
                                        {formatClock(
                                            currentOrder.window.end,
                                            snapshot.timezone
                                        )}
                                    </p>
                                    {view.selectedVisit ? (
                                        <p className="mt-1 font-medium">
                                            План{' '}
                                            {formatClock(
                                                view.selectedVisit.arrival_at,
                                                snapshot.timezone
                                            )}{' '}
                                            · работа{' '}
                                            {formatClock(
                                                view.selectedVisit.start_at,
                                                snapshot.timezone
                                            )}
                                        </p>
                                    ) : null}
                                </div>
                            ) : null}
                            {emergencyHighlights.length ? (
                                <section>
                                    <h3 className="mb-2 text-sm font-semibold">
                                        Изменённые аварии ·{' '}
                                        {emergencyHighlights.length}
                                    </h3>
                                    <div className="space-y-2">
                                        {emergencyHighlights.map((order) => {
                                            const assigned = assignments.get(
                                                order.id
                                            );
                                            const strict =
                                                strictAssignments.get(order.id);
                                            const change = plan.changes.find(
                                                (item) =>
                                                    item.order_id === order.id
                                            );
                                            const late = latenessByOrder.get(
                                                order.id
                                            );
                                            const previous = change?.before
                                                ? assignmentSummary(
                                                      change.before.engineer_id,
                                                      change.before.start_at,
                                                      snapshot.timezone
                                                  )
                                                : null;
                                            const strictSummary = strict
                                                ? assignmentSummary(
                                                      strict.engineerId,
                                                      strict.visit.start_at,
                                                      snapshot.timezone
                                                  )
                                                : null;
                                            const comparison = previous
                                                ? `Было в рабочем плане: ${previous}`
                                                : option.key === 'strict'
                                                  ? null
                                                  : strictSummary
                                                    ? `В варианте без опозданий: ${strictSummary}`
                                                    : 'В варианте без опозданий: без бригады';
                                            return (
                                                <button
                                                    type="button"
                                                    key={order.id}
                                                    data-order-id={order.id}
                                                    aria-pressed={
                                                        selectedOrderId ===
                                                        order.id
                                                    }
                                                    onClick={() =>
                                                        selectOrder(order.id)
                                                    }
                                                    className={[
                                                        'w-full rounded-[9px] border p-3 text-left',
                                                        selectedOrderId ===
                                                        order.id
                                                            ? 'border-amber-500 bg-amber-100'
                                                            : 'border-amber-300 bg-amber-50 hover:border-amber-500',
                                                    ].join(' ')}
                                                >
                                                    <span className="flex items-center justify-between gap-2">
                                                        <WorkTypeBadge type="emergency" />
                                                        {late ? (
                                                            <strong className="text-xs text-amber-950">
                                                                +
                                                                {elapsed(
                                                                    late.late_sec
                                                                )}
                                                            </strong>
                                                        ) : null}
                                                    </span>
                                                    <strong className="mt-2 block leading-snug">
                                                        {locations.get(
                                                            order.location_id
                                                        ) ?? order.id}
                                                    </strong>
                                                    <span className="mt-1 block text-muted-foreground">
                                                        Исходное окно{' '}
                                                        {formatClock(
                                                            order.window.start,
                                                            snapshot.timezone
                                                        )}
                                                        –
                                                        {formatClock(
                                                            order.window.end,
                                                            snapshot.timezone
                                                        )}
                                                    </span>
                                                    {assigned ? (
                                                        <span className="mt-1 block font-medium">
                                                            {displayEngineer(
                                                                assigned.engineerId
                                                            )}{' '}
                                                            · прибытие{' '}
                                                            {formatClock(
                                                                assigned.visit
                                                                    .arrival_at,
                                                                snapshot.timezone
                                                            )}{' '}
                                                            · начало{' '}
                                                            {formatClock(
                                                                assigned.visit
                                                                    .start_at,
                                                                snapshot.timezone
                                                            )}
                                                        </span>
                                                    ) : (
                                                        <span className="mt-1 block font-medium">
                                                            В этом варианте без
                                                            бригады
                                                        </span>
                                                    )}
                                                    {comparison ? (
                                                        <span className="mt-1 block text-muted-foreground">
                                                            {comparison}
                                                        </span>
                                                    ) : null}
                                                </button>
                                            );
                                        })}
                                    </div>
                                </section>
                            ) : null}
                            <section>
                                <h3 className="mb-2 text-sm font-semibold">
                                    Бригады · {plan.metrics.used_engineer_count}
                                </h3>
                                <div className="space-y-1">
                                    {plan.routes.map((route) => (
                                        <button
                                            type="button"
                                            key={route.engineer_id}
                                            aria-pressed={
                                                engineerId === route.engineer_id
                                            }
                                            onClick={() =>
                                                setEngineerId(
                                                    engineerId ===
                                                        route.engineer_id
                                                        ? null
                                                        : route.engineer_id
                                                )
                                            }
                                            className={[
                                                'flex w-full items-center gap-2 rounded-[8px]',
                                                'p-2 text-left hover:bg-muted',
                                                engineerId === route.engineer_id
                                                    ? 'bg-accent'
                                                    : '',
                                            ].join(' ')}
                                        >
                                            <i
                                                className="size-2.5 shrink-0 rounded-full"
                                                style={{
                                                    background: routeColor(
                                                        route.engineer_id
                                                    ),
                                                }}
                                            />
                                            <span className="min-w-0 flex-1 truncate">
                                                {displayEngineer(
                                                    route.engineer_id
                                                )}
                                                {reserve.has(route.engineer_id)
                                                    ? ' · резерв'
                                                    : ''}
                                            </span>
                                            <span className="text-muted-foreground">
                                                {formatCount(
                                                    route.visits.length,
                                                    [
                                                        'визит',
                                                        'визита',
                                                        'визитов',
                                                    ]
                                                )}{' '}
                                                ·{' '}
                                                {formatKm(
                                                    plan.metrics.per_engineer.find(
                                                        (item) =>
                                                            item.engineer_id ===
                                                            route.engineer_id
                                                    )?.distance_m ?? 0
                                                )}
                                            </span>
                                        </button>
                                    ))}
                                </div>
                                {engineerId ? (
                                    <ol className="mt-2 space-y-1 border-t border-border pt-2">
                                        {plan.routes
                                            .find(
                                                (route) =>
                                                    route.engineer_id ===
                                                    engineerId
                                            )
                                            ?.visits.map((visit, index) => {
                                                const order = orders.get(
                                                    visit.order_id
                                                );
                                                return (
                                                    <li
                                                        key={visit.order_id}
                                                        className="rounded-[8px] bg-muted/60 p-2"
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
                                                                snapshot.timezone
                                                            )}
                                                        </strong>
                                                        <span className="mt-1 block text-muted-foreground">
                                                            {order
                                                                ? locations.get(
                                                                      order.location_id
                                                                  )
                                                                : visit.order_id}
                                                        </span>
                                                    </li>
                                                );
                                            })}
                                    </ol>
                                ) : null}
                            </section>
                            {option.reserve_engineer_ids.length ? (
                                <section>
                                    <h3 className="mb-2 text-sm font-semibold">
                                        Вызвать с выходного
                                    </h3>
                                    {option.reserve_engineer_ids.map((id) => (
                                        <p
                                            key={id}
                                            className="mb-1 rounded-[8px] bg-muted p-2"
                                        >
                                            {displayEngineer(id)} ·{' '}
                                            {plan.routes.find(
                                                (route) =>
                                                    route.engineer_id === id
                                            )?.visits.length ?? 0}{' '}
                                            заявок
                                        </p>
                                    ))}
                                </section>
                            ) : null}
                            {snapshot.unlocated_orders?.length ? (
                                <section aria-label="Заявки без координат">
                                    <h3 className="mb-2 text-sm font-semibold">
                                        Без координат ·{' '}
                                        {snapshot.unlocated_orders.length}
                                    </h3>
                                    {snapshot.unlocated_orders.map((item) => (
                                        <article
                                            key={item.order.id}
                                            className="mb-2 rounded-[8px] border border-border p-2"
                                        >
                                            <WorkTypeBadge
                                                type={item.order.work_type}
                                            />
                                            <strong className="mt-1 block">
                                                {item.address}
                                            </strong>
                                            <p className="mt-1 text-muted-foreground">
                                                {item.message}
                                            </p>
                                        </article>
                                    ))}
                                </section>
                            ) : null}
                            {plan.unassigned.length ? (
                                <section>
                                    <h3 className="mb-2 text-sm font-semibold">
                                        Без бригады · {plan.unassigned.length}
                                    </h3>
                                    {plan.unassigned.map((item) => (
                                        <button
                                            type="button"
                                            key={item.order_id}
                                            onClick={() =>
                                                selectOrder(item.order_id)
                                            }
                                            aria-pressed={
                                                selectedOrderId ===
                                                item.order_id
                                            }
                                            className={[
                                                'mb-1 block w-full rounded-[8px] border border-border',
                                                'p-2 text-left hover:border-primary',
                                            ].join(' ')}
                                        >
                                            {orders.get(item.order_id) ? (
                                                <WorkTypeBadge
                                                    type={
                                                        orders.get(
                                                            item.order_id
                                                        )!.work_type
                                                    }
                                                />
                                            ) : null}
                                            <strong className="mt-1 block">
                                                {locations.get(
                                                    orders.get(item.order_id)
                                                        ?.location_id ?? ''
                                                ) ?? item.order_id}
                                            </strong>
                                            <span className="mt-1 block text-muted-foreground">
                                                {reasonCodeLabel[
                                                    item.reason_code
                                                ] ?? item.reason_code}
                                                {/[а-яё]/i.test(item.message)
                                                    ? ` · ${item.message}`
                                                    : ''}
                                            </span>
                                        </button>
                                    ))}
                                </section>
                            ) : null}
                            {plan.changes.length ? (
                                <section className="border-t border-border pt-3">
                                    <h3 className="mb-2 text-sm font-semibold">
                                        Изменения · {plan.changes.length}
                                    </h3>
                                    <div className="space-y-1">
                                        {plan.changes.map((change) => {
                                            const order = orders.get(
                                                change.order_id
                                            );
                                            const before = change.before
                                                ? `${displayEngineer(change.before.engineer_id)} · ${formatClock(
                                                      change.before.start_at,
                                                      snapshot.timezone
                                                  )}`
                                                : 'Без назначения';
                                            const after = change.after
                                                ? `${displayEngineer(change.after.engineer_id)} · ${formatClock(
                                                      change.after.start_at,
                                                      snapshot.timezone
                                                  )}`
                                                : 'Без назначения';
                                            return (
                                                <button
                                                    type="button"
                                                    key={`${change.order_id}-${change.reason}`}
                                                    onClick={() =>
                                                        setSelectedOrderId(
                                                            change.order_id
                                                        )
                                                    }
                                                    className={[
                                                        'w-full rounded-[8px] border border-border',
                                                        'p-2 text-left hover:bg-muted',
                                                    ].join(' ')}
                                                >
                                                    <strong className="block">
                                                        {order
                                                            ? locations.get(
                                                                  order.location_id
                                                              )
                                                            : change.order_id}
                                                    </strong>
                                                    <span className="mt-1 block text-muted-foreground">
                                                        {changeReasonLabel[
                                                            change.reason
                                                        ] ?? change.reason}
                                                    </span>
                                                    <span className="mt-1 block">
                                                        {before} → {after}
                                                    </span>
                                                </button>
                                            );
                                        })}
                                    </div>
                                </section>
                            ) : (
                                <p className="border-t border-border pt-3 text-muted-foreground">
                                    Назначения сохранились
                                </p>
                            )}
                        </div>
                        <footer className="shrink-0 border-t border-border bg-white px-4 py-3">
                            <Button
                                className="w-full rounded-[8px]"
                                disabled={accepting}
                                onClick={() => onAccept(option.key)}
                            >
                                {accepting
                                    ? 'Принимаем…'
                                    : option.key === 'original'
                                      ? 'Сохранить расписание'
                                      : 'Принять этот план'}
                            </Button>
                        </footer>
                    </aside>
                </div>
            </section>
        </div>
    );
};
