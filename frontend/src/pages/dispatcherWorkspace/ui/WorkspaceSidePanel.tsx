import {
    type Engineer,
    type Equipment,
    type Issue,
    type Order,
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
    panelTab: WorkspacePanelTab;
    ordersCount: number;
    crewsCount: number;
    orders: Order[];
    engineers: Engineer[];
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    engineerByOrder: Map<string, string>;
    unassigned: Map<string, UnassignedOrder>;
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
    panelTab,
    ordersCount,
    crewsCount,
    orders,
    engineers,
    selectedOrderId,
    selectedEngineerId,
    engineerByOrder,
    unassigned,
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
                'absolute top-[68px] right-4 bottom-4 z-20',
                'flex w-[368px] flex-col overflow-hidden rounded-[28px] bg-card'
            )}
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <Tabs
                value={panelTab}
                onValueChange={handleTabChange}
                className="mx-3 mt-3"
            >
                <TabsList className="grid grid-cols-2 p-1">
                    <TabsTrigger
                        value="orders"
                        className="py-2 data-[state=active]:bg-primary"
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
                        className="py-2 data-[state=active]:bg-primary"
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
                </TabsList>
            </Tabs>
            <div className="mt-3 flex min-h-0 flex-1 flex-col">
                {panelTab === 'crews' ? (
                    <CrewPanel
                        engineers={engineers}
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
                ) : (
                    <OrderPanel
                        orders={orders}
                        engineers={engineers}
                        selectedOrderId={selectedOrderId}
                        selectedEngineerId={selectedEngineerId}
                        engineerByOrder={engineerByOrder}
                        unassigned={unassigned}
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
                )}
            </div>
        </aside>
    );
};
