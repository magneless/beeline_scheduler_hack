import { type ChangeEvent, useEffect, useMemo, useState } from 'react';

import {
    type Engineer,
    type Order,
    type UnassignedOrder,
    type Visit,
} from 'shared/api/types/contracts';
import { statusLabel, workTypeLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { displayEngineer } from 'shared/lib/utils';

import { workspaceCopy } from '../lib/config';

import { type WorkspaceFilter } from './types';

const isClosed = (order: Order) =>
    order.status === 'completed' || order.status === 'cancelled';

type UseOrderPanelParams = {
    orders: Order[];
    engineerByOrder: Map<string, string>;
    unassigned: Map<string, UnassignedOrder>;
    deferredOrderIds: Set<string>;
    addressByOrder?: Record<string, string>;
    selectedEngineerId: TypeOrNull<string>;
    filter: WorkspaceFilter;
    engineers: Engineer[];
    visits: Map<string, Visit>;
};

export const useOrderPanel = ({
    orders,
    engineerByOrder,
    unassigned,
    deferredOrderIds,
    addressByOrder,
    selectedEngineerId,
    filter,
    engineers,
    visits,
}: UseOrderPanelParams) => {
    const [query, setQuery] = useState('');
    const [checkedIds, setCheckedIds] = useState<string[]>([]);
    useEffect(() => {
        setCheckedIds([]);
    }, [query, filter, selectedEngineerId]);

    const assignedCount = orders.filter(
        (order) => engineerByOrder.has(order.id) && !isClosed(order)
    ).length;
    const openCount = orders.filter(
        (order) => unassigned.has(order.id) && !deferredOrderIds.has(order.id)
    ).length;
    const deferredCount = orders.filter((order) =>
        deferredOrderIds.has(order.id)
    ).length;
    const closedCount = orders.filter(isClosed).length;
    const crewName = selectedEngineerId
        ? displayEngineer(selectedEngineerId)
        : undefined;
    const engineerById = useMemo(
        () => new Map(engineers.map((engineer) => [engineer.id, engineer])),
        [engineers]
    );
    const previousUnfinished = useMemo(() => {
        const byId = new Map(orders.map((order) => [order.id, order]));
        const firstOpen = new Map<string, Order>();
        const result = new Map<string, Order>();
        // Visit map follows the accepted route sequence, including work
        // hidden by the current search or filter.
        for (const visit of visits.values()) {
            const engineerId = engineerByOrder.get(visit.order_id);
            const order = byId.get(visit.order_id);
            if (!engineerId || !order) {
                continue;
            }
            const previous = firstOpen.get(engineerId);
            if (previous) {
                result.set(order.id, previous);
            } else if (!isClosed(order)) {
                firstOpen.set(engineerId, order);
            }
        }
        return result;
    }, [orders, visits, engineerByOrder]);

    const visible = useMemo(() => {
        const needle = query.trim().toLowerCase();
        const filtered = orders.filter((order) => {
            if (
                selectedEngineerId &&
                engineerByOrder.get(order.id) !== selectedEngineerId
            ) {
                return false;
            }
            if (
                filter === 'assigned' &&
                (!engineerByOrder.has(order.id) || isClosed(order))
            ) {
                return false;
            }
            if (
                filter === 'unassigned' &&
                (!unassigned.has(order.id) || deferredOrderIds.has(order.id))
            ) {
                return false;
            }
            if (filter === 'deferred' && !deferredOrderIds.has(order.id)) {
                return false;
            }

            if (filter === 'closed' && !isClosed(order)) {
                return false;
            }

            if (!needle) {
                return true;
            }

            const engineerId = engineerByOrder.get(order.id);
            const haystack = [
                workTypeLabel[order.work_type],
                addressByOrder?.[order.id],
                engineerId ? displayEngineer(engineerId) : workspaceCopy.noSlot,
                statusLabel[order.status],
                order.id,
            ]
                .filter(Boolean)
                .join(' ')
                .toLowerCase();

            return haystack.includes(needle);
        });

        return [...filtered].sort((left, right) => {
            const rankOf = (order: Order) => {
                if (isClosed(order)) {
                    return 4;
                }

                const mine =
                    selectedEngineerId !== null &&
                    engineerByOrder.get(order.id) === selectedEngineerId;

                if (mine) {
                    return 0;
                }

                if (unassigned.has(order.id)) {
                    return 1;
                }

                if (
                    order.work_type === 'emergency' ||
                    order.priority === 'urgent'
                ) {
                    return 2;
                }

                return 3;
            };

            const rank = rankOf(left) - rankOf(right);

            if (rank !== 0) {
                return rank;
            }

            return left.window.start.localeCompare(right.window.start);
        });
    }, [
        addressByOrder,
        engineerByOrder,
        filter,
        orders,
        query,
        selectedEngineerId,
        unassigned,
        deferredOrderIds,
    ]);

    // Selection only covers cancellable orders in the currently visible list.
    const selectableIds = visible
        .filter(
            (order) =>
                !isClosed(order) &&
                unassigned.has(order.id) &&
                unassigned.get(order.id)?.reason_code !==
                    'ADDRESS_UNRESOLVED' &&
                !engineerByOrder.has(order.id)
        )
        .map((order) => order.id);
    const selectedIds = selectableIds.filter((id) => checkedIds.includes(id));
    const allSelected =
        selectableIds.length > 0 && selectedIds.length === selectableIds.length;

    const emptyMessage = query
        ? workspaceCopy.orderEmptyQuery
        : filter === 'assigned'
          ? workspaceCopy.orderEmptyAssigned
          : filter === 'unassigned'
            ? workspaceCopy.orderEmptyOpen
            : filter === 'deferred'
              ? 'Нет заявок на разбор и перенос'
              : filter === 'closed'
                ? workspaceCopy.orderEmptyClosed
                : workspaceCopy.orderEmpty;

    const handleQueryChange = (event: ChangeEvent<HTMLInputElement>) => {
        setQuery(event.target.value);
    };

    return {
        query,
        assignedCount,
        openCount,
        deferredCount,
        closedCount,
        crewName,
        engineerById,
        visible,
        selectableIds,
        selectedIds,
        allSelected,
        toggleSelected: (id: string) => {
            if (selectableIds.includes(id)) {
                setCheckedIds((current) =>
                    current.includes(id)
                        ? current.filter((item) => item !== id)
                        : [...current, id]
                );
            }
        },
        toggleAll: () => setCheckedIds(allSelected ? [] : selectableIds),
        clearSelection: () => setCheckedIds([]),
        previousUnfinished,
        clearQuery: () => setQuery(''),
        emptyMessage,
        handleQueryChange,
        isClosed,
    };
};
