import { useEffect, useRef, useState } from 'react';

import {
    type CancelReason,
    type Engineer,
    type Equipment,
    type Order,
    type OrderStatus,
    type UnassignedOrder,
    type Visit,
} from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';
import { displayEngineer, formatClock } from 'shared/lib/utils';

import { assignmentFactors } from '../lib/assignmentFactors';
import { workspaceCopy } from '../lib/config';
import { alignToPlannedMinute, defaultExpectedEndAt } from '../lib/eventTime';

import { type WorkspaceEventInput } from './types';

type UseOrderRowParams = {
    order: Order;
    active: boolean;
    engineer?: Engineer;
    visit?: Visit;
    open?: UnassignedOrder;
    remaining?: Partial<Record<Equipment, number>>;
    timezone: string;
    defaultOccurredAt: string;
    onSelect: (id: TypeOrNull<string>) => void;
    onEvent?: (input: WorkspaceEventInput) => void;
};

export const useOrderRow = ({
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
}: UseOrderRowParams) => {
    const rowRef = useRef<HTMLDivElement>(null);
    const [occurredAt, setOccurredAt] = useState(defaultOccurredAt);
    const [error, setError] = useState('');

    useEffect(() => {
        setOccurredAt(defaultOccurredAt);
    }, [defaultOccurredAt]);

    useEffect(() => {
        setError('');
    }, [occurredAt, order.status]);

    const assignee = open
        ? workspaceCopy.noSlot
        : engineer
          ? displayEngineer(engineer.id)
          : workspaceCopy.unassigned;
    const factors =
        engineer && !open
            ? assignmentFactors(order, engineer, visit, timezone, remaining)
            : [];
    const closed = order.status === 'completed' || order.status === 'cancelled';

    const sendStatus = (
        status: Extract<
            OrderStatus,
            'sent' | 'en_route' | 'in_progress' | 'completed'
        >
    ) => {
        if (!engineer || !onEvent) {
            return;
        }

        const eventAt = alignToPlannedMinute(
            occurredAt,
            status === 'in_progress'
                ? visit?.start_at
                : status === 'completed'
                  ? visit?.end_at
                  : undefined
        );
        const at = Date.parse(eventAt);
        if (
            status === 'completed' &&
            order.execution?.started_at &&
            at <= Date.parse(order.execution.started_at)
        ) {
            setError(
                `Укажите время завершения позже начала работы (${formatClock(order.execution.started_at, timezone)}).`
            );
            return;
        }
        if (status === 'in_progress') {
            if (
                order.status !== 'in_progress' &&
                visit &&
                at <
                    Math.max(
                        Date.parse(visit.arrival_at),
                        Date.parse(order.window.start)
                    )
            ) {
                setError(
                    'Нельзя начать работу раньше прибытия бригады или начала окна заявки.'
                );
                return;
            }
        }
        setError('');
        setOccurredAt(eventAt);

        onEvent({
            kind: 'status',
            occurredAt: eventAt,
            status,
            expectedEndAt:
                status === 'in_progress'
                    ? defaultExpectedEndAt(eventAt, order.service_sec)
                    : undefined,
        });
    };

    const sendCancel = (reason: CancelReason) => {
        onEvent?.({ kind: 'cancel', occurredAt, reason });
    };

    const handleSelect = () => {
        onSelect(active ? null : order.id);
    };

    useEffect(() => {
        if (active) {
            rowRef.current?.scrollIntoView({
                block: 'nearest',
                behavior: 'smooth',
            });
        }
    }, [active]);

    return {
        rowRef,
        occurredAt,
        setOccurredAt,
        error,
        assignee,
        factors,
        closed,
        handleSelect,
        handleInProgress: () => sendStatus('in_progress'),
        handleCompleted: () => sendStatus('completed'),
        handleClientRefusal: () => sendCancel('client_refusal'),
        handleCannotPerform: () => sendCancel('cannot_perform'),
    };
};
