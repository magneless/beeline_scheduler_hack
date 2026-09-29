import { type DateTime } from 'luxon';

import { type WorkType } from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';

export type { PlanEventInput as WorkspaceEventInput } from 'features/applyPlanEvent';
export type { CrewPatchInput as EngineerPatchInput } from 'features/updateCrew';

export type WorkspaceFilter = 'all' | 'assigned' | 'unassigned' | 'closed';

export type CrewLoadFilter = 'all' | 'busy' | 'free' | 'reserve';

export type ScheduleBlock = {
    id: string;
    kind: 'travel' | 'wait' | 'work';
    start: DateTime;
    end: DateTime;
    orderId?: string;
    workType?: WorkType;
};

export type ScheduleLane = {
    engineerId: string;
    label?: string;
    start: DateTime;
    end: DateTime;
    blocks: ScheduleBlock[];
};

export type WorkspacePanelTab = 'orders' | 'crews' | 'both';

export type DispatcherWorkspaceStore = {
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    filter: WorkspaceFilter;
    panelOpen: boolean;
    scheduleOpen: boolean;
    panelTab: WorkspacePanelTab;
    selectOrder: (id: TypeOrNull<string>) => void;
    selectEngineer: (id: TypeOrNull<string>) => void;
    setFilter: (filter: WorkspaceFilter) => void;
    setPanelTab: (tab: WorkspacePanelTab) => void;
    togglePanel: () => void;
    toggleSchedule: () => void;
    resetSelection: () => void;
};
