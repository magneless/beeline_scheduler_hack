import {
    type Engineer,
    type Equipment,
    type Order,
    type OrderLateness,
    type UnassignedOrder,
    type Visit,
} from 'shared/api/types/contracts';
import {
    cancelReasonLabel,
    equipmentLabel,
    priorityLabel,
    reasonCodeLabel,
    statusLabel,
    workTypeLabel,
} from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { cn, formatClock, formatMinutes } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { Checkbox } from 'shared/ui/checkbox';

import { AssignmentFactors } from './AssignmentFactors';
import { EventTimeField } from './EventTimeField';
import { OrderAddressResolver } from './OrderAddressResolver';
import { workspaceCopy, workTypeDotClass } from '../lib/config';
import { useOrderRow } from '../model/useOrderRow';

import { type WorkspaceEventInput } from '../model/types';

type OrderRowProps = {
    order: Order;
    active: boolean;
    mine: boolean;
    engineer?: Engineer;
    visit?: Visit;
    open?: UnassignedOrder;
    lateness?: OrderLateness;
    inTransit?: boolean;
    address?: string;
    remaining?: Partial<Record<Equipment, number>>;
    needsEnd?: boolean;
    timezone: string;
    defaultOccurredAt: string;
    statusPending?: boolean;
    checked?: boolean;
    onToggleSelection?: () => void;
    onSelect: (id: TypeOrNull<string>) => void;
    onEvent?: (input: WorkspaceEventInput) => void;
};

export const OrderRow = ({
    order,
    active,
    mine,
    engineer,
    visit,
    open,
    lateness,
    inTransit,
    address,
    remaining,
    needsEnd,
    timezone,
    defaultOccurredAt,
    statusPending,
    checked,
    onToggleSelection,
    onSelect,
    onEvent,
}: OrderRowProps) => {
    const row = useOrderRow({
        order,
        active,
        engineer,
        visit,
        open,
        remaining,
        timezone,
        defaultOccurredAt,
        onSelect,
        onEvent,
    });
    const visitLabel = order.status === 'in_progress' ? 'По плану' : 'Визит';

    return (
        <article
            ref={row.rowRef}
            data-order-id={order.id}
            className={cn(
                'rounded-[12px] border px-3 py-2.5',
                active || checked
                    ? 'border-primary bg-accent'
                    : mine
                      ? 'border-blue-100 bg-accent/50'
                      : 'border-transparent hover:border-border hover:bg-background'
            )}
        >
            <div className="flex items-start gap-2">
                {onToggleSelection ? (
                    <Checkbox
                        className="mt-1"
                        aria-label={`Выбрать заявку ${order.id}`}
                        checked={checked}
                        onCheckedChange={onToggleSelection}
                        disabled={statusPending}
                    />
                ) : null}
                <Button
                    type="button"
                    variant="ghost"
                    className={cn(
                        'h-auto min-w-0 flex-1 items-start justify-start gap-2.5',
                        'px-0 py-0 text-left whitespace-normal'
                    )}
                    onClick={row.handleSelect}
                    aria-pressed={active}
                >
                    <span
                        className={cn(
                            'mt-1.5 size-2 shrink-0 rounded-full',
                            workTypeDotClass[order.work_type]
                        )}
                    />
                    <span className="min-w-0 flex-1">
                        <span className="block">
                            {address ? (
                                <span className="block line-clamp-2 text-sm font-semibold leading-snug text-foreground">
                                    {address.replace(
                                        /^(?:г\.?\s*)?(?:Город\s+)?Москва,?\s*/i,
                                        ''
                                    )}
                                </span>
                            ) : null}
                            <span className="mt-0.5 flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
                                <span className="text-[11px] font-medium text-muted-foreground">
                                    {workTypeLabel[order.work_type]}
                                    {order.priority === 'urgent' ? (
                                        <span className="ml-1.5 text-[11px] font-semibold text-destructive">
                                            {priorityLabel.urgent}
                                        </span>
                                    ) : null}
                                </span>
                                <span
                                    className={cn(
                                        'shrink-0 text-[11px] font-semibold',
                                        open
                                            ? 'text-destructive'
                                            : 'text-muted-foreground'
                                    )}
                                >
                                    {row.assignee}
                                </span>
                            </span>
                            <span className="mt-0.5 block text-[11px] text-muted-foreground">
                                Окно {formatClock(order.window.start, timezone)}
                                –{formatClock(order.window.end, timezone)}
                                {` · ${formatMinutes(order.service_sec)}`}
                            </span>
                        </span>
                    </span>
                </Button>
            </div>
            {open && !active ? (
                <p className="mt-1 ml-4.5 text-[11px] text-destructive">
                    {open.message}
                </p>
            ) : null}
            {active ? (
                <div className="mt-2 ml-4.5 space-y-2">
                    <p className="text-[11px] text-muted-foreground">
                        {visit
                            ? `${visitLabel} ${formatClock(visit.start_at, timezone)}–${formatClock(
                                  visit.end_at,
                                  timezone
                              )} · ${inTransit ? statusLabel.en_route : statusLabel[order.status]}`
                            : inTransit
                              ? statusLabel.en_route
                              : statusLabel[order.status]}
                    </p>
                    {order.status === 'in_progress' &&
                    order.execution?.started_at ? (
                        <p className="text-[11px] text-muted-foreground">
                            Начато фактически{' '}
                            {formatClock(order.execution.started_at, timezone)}
                        </p>
                    ) : null}
                    {lateness ? (
                        <p className="rounded-[8px] bg-amber-50 px-3 py-2 text-xs text-amber-950">
                            Окно до {formatClock(lateness.window.end, timezone)}{' '}
                            · прибытие{' '}
                            {formatClock(lateness.arrival_at, timezone)} ·
                            опоздание {formatMinutes(lateness.late_sec)}
                        </p>
                    ) : null}
                    {open ? (
                        <p className="rounded-[16px] bg-card px-3 py-2 text-xs">
                            <strong className="block font-semibold">
                                {reasonCodeLabel[open.reason_code] ??
                                    'Требуется проверка назначения'}
                            </strong>
                            <span className="mt-1 block text-muted-foreground">
                                {/[а-яё]/i.test(open.message)
                                    ? open.message
                                    : 'Проверьте доступность бригад и окно визита. ' +
                                      'Если выполнить заявку невозможно, укажите причину ниже.'}
                            </span>
                        </p>
                    ) : null}
                    {open?.reason_code === 'ADDRESS_UNRESOLVED' ? (
                        <OrderAddressResolver
                            order={order}
                            address={address}
                            occurredAt={defaultOccurredAt}
                            pending={statusPending}
                            onEvent={onEvent}
                        />
                    ) : null}
                    <details className="text-xs">
                        <summary className="cursor-pointer py-1 text-muted-foreground">
                            Условия назначения
                        </summary>
                        <dl className="rounded-lg border border-border bg-card px-3 py-2 text-xs">
                            <dt className="font-semibold">
                                Требуется из инвентаря
                            </dt>
                            <dd className="mt-1 text-muted-foreground">
                                {Object.entries(order.equipment_required ?? {})
                                    .filter(([, count]) => count && count > 0)
                                    .map(
                                        ([equipment, count]) =>
                                            `${equipmentLabel[equipment as Equipment] ?? equipment} — ${count} шт.`
                                    )
                                    .join(', ') || 'Оборудование не требуется'}
                            </dd>
                        </dl>
                        {row.factors.length ? (
                            <AssignmentFactors factors={row.factors} />
                        ) : null}
                    </details>
                    {needsEnd ? (
                        <p
                            className={cn(
                                'rounded-[16px] bg-destructive/10 px-3 py-2',
                                'text-xs font-semibold text-destructive'
                            )}
                        >
                            {workspaceCopy.needsEnd}
                        </p>
                    ) : null}
                    {!row.closed &&
                    onEvent &&
                    open?.reason_code !== 'ADDRESS_UNRESOLVED' ? (
                        <div className="space-y-2">
                            <EventTimeField
                                label="Время события"
                                value={row.occurredAt}
                                timezone={timezone}
                                onChange={row.setOccurredAt}
                            />
                            {row.error ? (
                                <p
                                    role="alert"
                                    className="text-xs text-destructive"
                                >
                                    {row.error}
                                </p>
                            ) : null}
                            <div className="flex flex-wrap gap-2">
                                {['active', 'sent', 'en_route'].includes(
                                    order.status
                                ) && engineer ? (
                                    <Button
                                        size="sm"
                                        disabled={
                                            statusPending || !engineer.available
                                        }
                                        onClick={row.handleInProgress}
                                    >
                                        Начать работу
                                    </Button>
                                ) : null}
                                {order.status === 'in_progress' && engineer ? (
                                    <Button
                                        size="sm"
                                        disabled={statusPending}
                                        onClick={row.handleCompleted}
                                    >
                                        {statusLabel.completed}
                                    </Button>
                                ) : null}
                                {order.status !== 'in_progress' ? (
                                    <>
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={statusPending}
                                            onClick={row.handleClientRefusal}
                                        >
                                            {cancelReasonLabel.client_refusal}
                                        </Button>
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={statusPending}
                                            onClick={row.handleCannotPerform}
                                        >
                                            {cancelReasonLabel.cannot_perform}
                                        </Button>
                                    </>
                                ) : null}
                            </div>
                        </div>
                    ) : null}
                </div>
            ) : null}
        </article>
    );
};
