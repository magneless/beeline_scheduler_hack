import { type ReactNode } from 'react';
import { ArrowUpRight } from 'lucide-react';

import {
    type Engineer,
    type Equipment,
    type Issue,
    type Order,
    type OrderLateness,
    type UnassignedOrder,
    type Visit,
} from 'shared/api';
import { type TypeOrNull } from 'shared/lib/types';
import { cn } from 'shared/lib/utils';
import { Tabs, TabsList, TabsTrigger } from 'shared/ui/tabs';

import { CrewPanel } from './CrewPanel';
import { OrderPanel } from './OrderPanel';
import { workspaceCopy } from '../lib/config';

import {
    type EngineerPatchInput,
    type WorkspaceEventInput,
    type WorkspaceFilter,
    type WorkspacePanelTab,
} from '../model/types';

type WorkspaceSidePanelProps = {
    children?: ReactNode;
    hasPlan: boolean;
    asOf?: string;
    onShowUnassigned: () => void;
    panelTab: WorkspacePanelTab;
    ordersCount: number;
    crewsCount: number;
    orders: Order[];
    engineers: Engineer[];
    scenarioId?: string;
    revision?: number;
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    engineerByOrder: Map<string, string>;
    unassigned: Map<string, UnassignedOrder>;
    lateness?: OrderLateness[];
    inTransitOrderIds?: Set<string>;
    deferredOrderIds: Set<string>;
    visits: Map<string, Visit>;
    timezone: string;
    date: string;
    addressByOrder: Record<string, string>;
    remaining?: Record<string, Partial<Record<Equipment, number>>>;
    issues: Issue[];
    defaultOccurredAt: string;
    filter: WorkspaceFilter;
    distances?: Record<string, number>;
    baselineDistances?: Record<string, number>;
    assignedCounts?: Record<string, number>;
    canEditEngineers: boolean;
    canEvent: boolean;
    pending: boolean;
    statusPending: boolean;
    onPanelTab: (tab: WorkspacePanelTab) => void;
    onFilter: (filter: WorkspaceFilter) => void;
    onSelectOrder: (id: TypeOrNull<string>) => void;
    onSelectEngineer: (id: TypeOrNull<string>) => void;
    onClearCrew: () => void;
    onOrderEvent?: (input: WorkspaceEventInput) => void;
    onPatchEngineer?: (engineerId: string, patch: EngineerPatchInput) => void;
    onUnavailable?: (engineerId: string, occurredAt: string) => void;
};

export const WorkspaceSidePanel = ({
    children,
    hasPlan,
    asOf,
    onShowUnassigned,
    panelTab,
    ordersCount,
    crewsCount,
    orders,
    engineers,
    scenarioId,
    revision,
    selectedOrderId,
    selectedEngineerId,
    engineerByOrder,
    unassigned,
    lateness,
    inTransitOrderIds,
    deferredOrderIds,
    visits,
    timezone,
    date,
    addressByOrder,
    remaining,
    issues,
    defaultOccurredAt,
    filter,
    distances,
    baselineDistances,
    assignedCounts,
    canEditEngineers,
    canEvent,
    pending,
    statusPending,
    onPanelTab,
    onFilter,
    onSelectOrder,
    onSelectEngineer,
    onClearCrew,
    onOrderEvent,
    onPatchEngineer,
    onUnavailable,
}: WorkspaceSidePanelProps) => {
    const handleTabChange = (value: string) => {
        onPanelTab(value as WorkspacePanelTab);
    };

    return (
        <aside
            className={cn(
                'relative z-10 flex min-h-0 shrink-0 flex-col overflow-hidden border-r border-border bg-white',
                panelTab === 'both'
                    ? 'w-[660px] max-lg:h-[580px] max-lg:w-full max-sm:h-[900px]'
                    : 'w-[340px] max-lg:w-[300px] max-lg:w-full max-lg:max-h-[58vh]'
            )}
            aria-label="Рабочая панель диспетчера"
        >
            {hasPlan && unassigned.size > 0 ? (
                <button
                    type="button"
                    onClick={onShowUnassigned}
                    className="mx-4 mt-4 flex items-center justify-between rounded-[8px] bg-accent px-3 py-2 text-sm"
                    aria-label={`Разобрать заявки без бригады: ${unassigned.size}`}
                >
                    <span>{unassigned.size} без бригады</span>
                    <ArrowUpRight className="size-4" />
                </button>
            ) : null}
            <Tabs
                value={panelTab}
                onValueChange={handleTabChange}
                className="mx-4 mt-4 shrink-0"
            >
                <TabsList className="grid grid-cols-3 rounded-[8px] bg-muted p-1">
                    <TabsTrigger
                        value="orders"
                        className={[
                            'rounded-md py-2 text-xs data-[state=active]:bg-primary',
                            'data-[state=active]:text-foreground',
                        ].join(' ')}
                    >
                        {workspaceCopy.ordersTab}
                        <span
                            className={cn(
                                'min-w-5 rounded-full bg-card px-1.5',
                                'text-[10px] font-bold tabular-nums'
                            )}
                        >
                            {ordersCount}
                        </span>
                    </TabsTrigger>
                    <TabsTrigger
                        value="crews"
                        className={[
                            'rounded-md py-2 text-xs data-[state=active]:bg-primary',
                            'data-[state=active]:text-foreground',
                        ].join(' ')}
                    >
                        {workspaceCopy.crewsTab}
                        <span
                            className={cn(
                                'min-w-5 rounded-full bg-card px-1.5',
                                'text-[10px] font-bold tabular-nums'
                            )}
                        >
                            {crewsCount}
                        </span>
                    </TabsTrigger>
                    <TabsTrigger
                        value="both"
                        className="rounded-md py-2 text-xs data-[state=active]:bg-primary"
                    >
                        Вместе
                    </TabsTrigger>
                </TabsList>
            </Tabs>
            <div
                className={cn(
                    'mt-3 min-h-0 flex-1',
                    panelTab === 'both'
                        ? 'grid grid-cols-2 max-sm:grid-cols-1 max-sm:grid-rows-2'
                        : 'flex flex-col'
                )}
            >
                {panelTab !== 'orders' ? (
                    <section
                        aria-label="Бригады"
                        className="order-2 flex min-h-0 min-w-0 flex-col"
                    >
                        {panelTab === 'both' ? (
                            <h2 className="px-3 pb-3 text-sm font-bold">
                                Бригады · {crewsCount}
                            </h2>
                        ) : null}
                        <CrewPanel
                            comparisonOrder={orders.find(
                                (order) =>
                                    order.id === selectedOrderId &&
                                    unassigned.has(order.id) &&
                                    unassigned.get(order.id)?.reason_code !==
                                        'ADDRESS_UNRESOLVED'
                            )}
                            comparisonAddress={
                                selectedOrderId
                                    ? addressByOrder[selectedOrderId]
                                    : undefined
                            }
                            asOf={asOf}
                            engineers={engineers}
                            scenarioId={scenarioId ?? ''}
                            revision={revision ?? 0}
                            selectedEngineerId={selectedEngineerId}
                            timezone={timezone}
                            date={date}
                            remaining={remaining}
                            distances={distances}
                            baselineDistances={baselineDistances}
                            assignedCounts={assignedCounts}
                            canEdit={canEditEngineers}
                            canEvent={canEvent}
                            defaultOccurredAt={defaultOccurredAt}
                            pending={pending}
                            onSelect={onSelectEngineer}
                            onPatch={onPatchEngineer}
                            onUnavailable={onUnavailable}
                        />
                    </section>
                ) : null}
                {panelTab !== 'crews' ? (
                    <section
                        aria-label="Заявки"
                        className={cn(
                            'order-1 flex min-h-0 min-w-0 flex-col',
                            panelTab === 'both' &&
                                'border-r border-border max-sm:mb-3 max-sm:border-r-0 max-sm:border-b'
                        )}
                    >
                        {panelTab === 'both' ? (
                            <h2 className="px-3 pb-3 text-sm font-bold">
                                Заявки · {ordersCount}
                            </h2>
                        ) : null}
                        <OrderPanel
                            key={scenarioId}
                            orders={orders}
                            engineers={engineers}
                            selectedOrderId={selectedOrderId}
                            selectedEngineerId={
                                panelTab === 'both' ? null : selectedEngineerId
                            }
                            engineerByOrder={engineerByOrder}
                            unassigned={unassigned}
                            lateness={lateness}
                            inTransitOrderIds={inTransitOrderIds}
                            deferredOrderIds={deferredOrderIds}
                            visits={visits}
                            timezone={timezone}
                            addressByOrder={addressByOrder}
                            remaining={remaining}
                            issues={issues}
                            defaultOccurredAt={defaultOccurredAt}
                            filter={filter}
                            onFilter={onFilter}
                            onSelect={onSelectOrder}
                            onClearCrew={onClearCrew}
                            onEvent={onOrderEvent}
                            statusPending={statusPending}
                        />
                    </section>
                ) : null}
            </div>
            {children}
        </aside>
    );
};
