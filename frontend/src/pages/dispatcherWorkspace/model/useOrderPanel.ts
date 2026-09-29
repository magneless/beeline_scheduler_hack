import { type ChangeEvent, useEffect, useMemo, useState } from 'react';

import {
    type Engineer,
    type Order,
    type UnassignedOrder,
    type Visit,
    type WorkType,
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
    addressByOrder,
    selectedEngineerId,
    filter,
    engineers,
}: UseOrderPanelParams) => {
    const [query, setQuery] = useState('');
    const [workType, setWorkType] = useState<WorkType | 'all'>('all');
    const [checkedIds, setCheckedIds] = useState<string[]>([]);
    useEffect(() => {
        setCheckedIds([]);
    }, [query, filter, selectedEngineerId, workType]);

    const scopedOrders = orders.filter(
        (order) =>
            (workType === 'all' || order.work_type === workType) &&
            (!selectedEngineerId ||
                engineerByOrder.get(order.id) === selectedEngineerId)
    );
    const assignedCount = scopedOrders.filter(
        (order) => engineerByOrder.has(order.id) && !isClosed(order)
    ).length;
    const openCount = scopedOrders.filter(
        (order) => !isClosed(order) && unassigned.has(order.id)
    ).length;
    const closedCount = scopedOrders.filter(isClosed).length;
    const crewName = selectedEngineerId
        ? displayEngineer(selectedEngineerId)
        : undefined;
    const engineerById = useMemo(
        () => new Map(engineers.map((engineer) => [engineer.id, engineer])),
        [engineers]
    );
    const visible = useMemo(() => {
        const needle = query.trim().toLowerCase();
        const filtered = orders.filter((order) => {
            if (workType !== 'all' && order.work_type !== workType) {
                return false;
            }
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
                (isClosed(order) || !unassigned.has(order.id))
            ) {
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
        workType,
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

    const emptyMessage =
        workType !== 'all' || selectedEngineerId
            ? 'Нет заявок с выбранными фильтрами'
            : query
              ? workspaceCopy.orderEmptyQuery
              : filter === 'assigned'
                ? workspaceCopy.orderEmptyAssigned
                : filter === 'unassigned'
                  ? workspaceCopy.orderEmptyOpen
                  : filter === 'closed'
                    ? workspaceCopy.orderEmptyClosed
                    : workspaceCopy.orderEmpty;

    const handleQueryChange = (event: ChangeEvent<HTMLInputElement>) => {
        setQuery(event.target.value);
    };

    return {
        query,
        workType,
        setWorkType,
        assignedCount,
        totalCount: scopedOrders.length,
        openCount,
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
        clearQuery: () => setQuery(''),
        emptyMessage,
        handleQueryChange,
        isClosed,
    };
};
