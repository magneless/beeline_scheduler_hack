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
import { displayEngineer } from 'shared/lib/utils';

import { assignmentFactors } from '../lib/assignmentFactors';
import { workspaceCopy } from '../lib/config';
import { defaultExpectedEndAt } from '../lib/eventTime';

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
    const [expectedEndAt, setExpectedEndAt] = useState(
        order.execution?.expected_end_at ??
            defaultExpectedEndAt(defaultOccurredAt, visit?.end_at)
    );

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

        onEvent({
            kind: 'status',
            occurredAt,
            status,
            expectedEndAt:
                status === 'in_progress' && expectedEndAt
                    ? expectedEndAt
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
        expectedEndAt,
        setExpectedEndAt,
        assignee,
        factors,
        closed,
        handleSelect,
        handleSent: () => sendStatus('sent'),
        handleEnRoute: () => sendStatus('en_route'),
        handleInProgress: () => sendStatus('in_progress'),
        handleCompleted: () => sendStatus('completed'),
        handleClientRefusal: () => sendCancel('client_refusal'),
        handleCannotPerform: () => sendCancel('cannot_perform'),
    };
};
