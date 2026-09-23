import { Search, X } from 'lucide-react';

import {
    type Engineer,
    type Equipment,
    type Issue,
    type Order,
    type UnassignedOrder,
    type Visit,
} from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';
import { cn } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { Input } from 'shared/ui/input';
import { Tabs, TabsList, TabsTrigger } from 'shared/ui/tabs';

import { OrderRow } from './OrderRow';
import { workspaceCopy } from '../lib/config';
import { useOrderPanel } from '../model/useOrderPanel';

import { type WorkspaceEventInput, type WorkspaceFilter } from '../model/types';

type OrderPanelProps = {
    orders: Order[];
    engineers: Engineer[];
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    engineerByOrder: Map<string, string>;
    unassigned: Map<string, UnassignedOrder>;
    visits: Map<string, Visit>;
    timezone: string;
    addressByOrder?: Record<string, string>;
    remaining?: Record<string, Partial<Record<Equipment, number>>>;
    issues?: Issue[];
    defaultOccurredAt: string;
    filter: WorkspaceFilter;
    onFilter: (filter: WorkspaceFilter) => void;
    onSelect: (id: TypeOrNull<string>) => void;
    onClearCrew: () => void;
    onEvent?: (input: WorkspaceEventInput) => void;
    statusPending?: boolean;
};

export const OrderPanel = ({
    orders,
    engineers,
    selectedOrderId,
    selectedEngineerId,
    engineerByOrder,
    unassigned,
    visits,
    timezone,
    addressByOrder,
    remaining,
    issues,
    defaultOccurredAt,
    filter,
    onFilter,
    onSelect,
    onClearCrew,
    onEvent,
    statusPending,
}: OrderPanelProps) => {
    const panel = useOrderPanel({
        orders,
        engineers,
        engineerByOrder,
        unassigned,
        addressByOrder,
        selectedEngineerId,
        filter,
    });

    const handleFilterChange = (value: string) => {
        onFilter(value as WorkspaceFilter);
    };

    return (
        <div className="flex h-full min-h-0 flex-col">
            <div className="shrink-0 space-y-2 px-3">
                <div className="relative">
                    <Search
                        className={cn(
                            'pointer-events-none absolute top-1/2 left-3 size-3.5',
                            '-translate-y-1/2 text-muted-foreground'
                        )}
                    />
                    <Input
                        value={panel.query}
                        placeholder={workspaceCopy.orderSearch}
                        aria-label="Поиск заявок"
                        className="h-8 bg-slate-50 pl-8 text-xs"
                        onChange={panel.handleQueryChange}
                    />
                </div>
                <Tabs value={filter} onValueChange={handleFilterChange}>
                    <TabsList className="grid grid-cols-3">
                        <TabsTrigger value="all">
                            {workspaceCopy.orderFilterAll}
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {orders.length}
                            </span>
                        </TabsTrigger>
                        <TabsTrigger value="unassigned">
                            Без бригады
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {panel.openCount}
                            </span>
                        </TabsTrigger>
                        <TabsTrigger
                            value="closed"
                            title="Выполненные и отменённые заявки"
                        >
                            Закрыты
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {panel.closedCount}
                            </span>
                        </TabsTrigger>
                    </TabsList>
                </Tabs>
                {panel.crewName ? (
                    <Button
                        type="button"
                        variant="ghost"
                        className={cn(
                            'h-auto w-full justify-between gap-2 rounded-[16px]',
                            'bg-accent px-3 py-2 text-left whitespace-normal hover:bg-accent'
                        )}
                        onClick={onClearCrew}
                    >
                        <span className="min-w-0">
                            <span className="block truncate text-xs font-semibold">
                                {workspaceCopy.crewOnMap(panel.crewName)}
                            </span>
                            <span className="block text-[11px] text-muted-foreground">
                                {workspaceCopy.crewOnMapHint}
                            </span>
                        </span>
                        <X className="size-3.5 shrink-0" />
                    </Button>
                ) : null}
            </div>
            <div className="mt-2 min-h-0 flex-1 overflow-y-auto px-2 pb-3">
                {panel.visible.length ? (
                    <div className="flex flex-col gap-1">
                        {panel.visible.map((order) => {
                            const engineerId = engineerByOrder.get(order.id);

                            return (
                                <OrderRow
                                    key={order.id}
                                    order={order}
                                    active={order.id === selectedOrderId}
                                    mine={
                                        selectedEngineerId
                                            ? engineerId === selectedEngineerId
                                            : false
                                    }
                                    engineer={
                                        engineerId
                                            ? panel.engineerById.get(engineerId)
                                            : undefined
                                    }
                                    visit={visits.get(order.id)}
                                    open={unassigned.get(order.id)}
                                    address={addressByOrder?.[order.id]}
                                    remaining={
                                        remaining
                                            ? engineerId
                                                ? (remaining[engineerId] ?? {})
                                                : undefined
                                            : undefined
                                    }
                                    needsEnd={issues?.some(
                                        (issue) =>
                                            issue.code ===
                                                'EXECUTION_STATE_REQUIRED' &&
                                            issue.entity_id === order.id
                                    )}
                                    timezone={timezone}
                                    defaultOccurredAt={defaultOccurredAt}
                                    statusPending={statusPending}
                                    onSelect={onSelect}
                                    onEvent={onEvent}
                                />
                            );
                        })}
                    </div>
                ) : (
                    <p className="px-3 py-10 text-center text-sm text-muted-foreground">
                        {panel.emptyMessage}
                    </p>
                )}
            </div>
        </div>
    );
};
