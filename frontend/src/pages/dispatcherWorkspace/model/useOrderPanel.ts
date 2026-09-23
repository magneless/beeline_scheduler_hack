import { type ChangeEvent, useMemo, useState } from 'react';

import {
    type Engineer,
    type Order,
    type UnassignedOrder,
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

    const openCount = orders.filter((order) => unassigned.has(order.id)).length;
    const closedCount = orders.filter(isClosed).length;
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
            if (filter === 'unassigned' && !unassigned.has(order.id)) {
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
    ]);

    const emptyMessage = query
        ? workspaceCopy.orderEmptyQuery
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
        openCount,
        closedCount,
        crewName,
        engineerById,
        visible,
        emptyMessage,
        handleQueryChange,
        isClosed,
    };
};
