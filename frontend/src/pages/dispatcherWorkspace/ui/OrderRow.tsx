import {
    type Engineer,
    type Equipment,
    type Order,
    type UnassignedOrder,
    type Visit,
} from 'shared/api/types/contracts';
import {
    cancelReasonLabel,
    priorityLabel,
    reasonCodeLabel,
    statusLabel,
    workTypeLabel,
} from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { cn, formatClock, formatMinutes } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { AssignmentFactors } from './AssignmentFactors';
import { EventTimeField } from './EventTimeField';
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
    address?: string;
    remaining?: Partial<Record<Equipment, number>>;
    needsEnd?: boolean;
    timezone: string;
    defaultOccurredAt: string;
    statusPending?: boolean;
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
    address,
    remaining,
    needsEnd,
    timezone,
    defaultOccurredAt,
    statusPending,
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

    return (
        <article
            ref={row.rowRef}
            data-order-id={order.id}
            className={cn(
                'rounded-[12px] border px-3 py-2.5',
                active
                    ? 'border-primary bg-accent'
                    : mine
                      ? 'border-blue-100 bg-accent/50'
                      : 'border-transparent hover:border-border hover:bg-background'
            )}
        >
            <Button
                type="button"
                variant="ghost"
                className="h-auto w-full items-start justify-start gap-2.5 px-0 py-0 text-left whitespace-normal"
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
                        <span className="mt-0.5 flex items-center justify-between gap-2">
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
                            Окно {formatClock(order.window.start, timezone)}–
                            {formatClock(order.window.end, timezone)}
                            {` · ${formatMinutes(order.service_sec)}`}
                        </span>
                    </span>
                </span>
            </Button>
            {open && !active ? (
                <p className="mt-1 ml-4.5 text-[11px] text-destructive">
                    {reasonCodeLabel[open.reason_code] ?? open.message}
                </p>
            ) : null}
            {active ? (
                <div className="mt-2 ml-4.5 space-y-2">
                    <p className="text-[11px] text-muted-foreground">
                        {visit
                            ? `Визит ${formatClock(visit.start_at, timezone)}–${formatClock(
                                  visit.end_at,
                                  timezone
                              )} · ${statusLabel[order.status]}`
                            : statusLabel[order.status]}
                    </p>
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
                    {row.factors.length ? (
                        <details className="text-xs">
                            <summary className="cursor-pointer py-1 text-muted-foreground">
                                Условия назначения
                            </summary>
                            <AssignmentFactors factors={row.factors} />
                        </details>
                    ) : null}
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
                    {!row.closed && onEvent ? (
                        <div className="space-y-2">
                            <EventTimeField
                                value={row.occurredAt}
                                timezone={timezone}
                                onChange={row.setOccurredAt}
                            />
                            {order.status === 'in_progress' ? (
                                <EventTimeField
                                    label="Ожидаемое окончание"
                                    value={row.expectedEndAt}
                                    timezone={timezone}
                                    onChange={row.setExpectedEndAt}
                                />
                            ) : null}
                            <div className="flex flex-wrap gap-2">
                                {order.status === 'active' && engineer ? (
                                    <Button
                                        size="sm"
                                        disabled={statusPending}
                                        onClick={row.handleSent}
                                    >
                                        {statusLabel.sent}
                                    </Button>
                                ) : null}
                                {order.status === 'sent' && engineer ? (
                                    <>
                                        <Button
                                            size="sm"
                                            disabled={statusPending}
                                            onClick={row.handleEnRoute}
                                        >
                                            {statusLabel.en_route}
                                        </Button>
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={statusPending}
                                            onClick={row.handleInProgress}
                                        >
                                            {workspaceCopy.alreadyOnSite}
                                        </Button>
                                    </>
                                ) : null}
                                {order.status === 'en_route' && engineer ? (
                                    <Button
                                        size="sm"
                                        disabled={statusPending}
                                        onClick={row.handleInProgress}
                                    >
                                        {statusLabel.in_progress}
                                    </Button>
                                ) : null}
                                {order.status === 'in_progress' && engineer ? (
                                    <>
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={statusPending}
                                            onClick={row.handleInProgress}
                                        >
                                            {workspaceCopy.updateEstimate}
                                        </Button>
                                        <Button
                                            size="sm"
                                            disabled={statusPending}
                                            onClick={row.handleCompleted}
                                        >
                                            {statusLabel.completed}
                                        </Button>
                                    </>
                                ) : null}
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
                            </div>
                        </div>
                    ) : null}
                </div>
            ) : null}
        </article>
    );
};
