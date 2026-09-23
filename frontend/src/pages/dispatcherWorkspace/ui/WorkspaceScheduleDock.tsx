import { type DateTime } from 'luxon';

import { type TypeOrNull } from 'shared/lib/types';
import { cn } from 'shared/lib/utils';

import { ScheduleBoard } from './ScheduleBoard';

import { type ScheduleLane } from '../model/types';

type WorkspaceScheduleDockProps = {
    panelOpen: boolean;
    lanes: ScheduleLane[];
    focusAt: DateTime;
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    onSelectOrder: (id: string) => void;
    onSelectEngineer: (id: TypeOrNull<string>) => void;
};

export const WorkspaceScheduleDock = ({
    panelOpen,
    lanes,
    focusAt,
    selectedOrderId,
    selectedEngineerId,
    onSelectOrder,
    onSelectEngineer,
}: WorkspaceScheduleDockProps) => (
    <div
        className={cn(
            'pointer-events-none absolute bottom-4 left-4 z-20',
            panelOpen ? 'right-[396px]' : 'right-4'
        )}
    >
        <div
            className="pointer-events-auto overflow-visible rounded-[28px] bg-card"
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <ScheduleBoard
                lanes={lanes}
                focusAt={focusAt}
                selectedOrderId={selectedOrderId}
                selectedEngineerId={selectedEngineerId}
                onSelectOrder={onSelectOrder}
                onSelectEngineer={onSelectEngineer}
            />
        </div>
    </div>
);
