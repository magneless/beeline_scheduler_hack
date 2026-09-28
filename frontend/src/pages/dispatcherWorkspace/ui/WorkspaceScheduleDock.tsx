import { type DateTime } from 'luxon';

import { type TypeOrNull } from 'shared/lib/types';

import { ScheduleBoard } from './ScheduleBoard';

import { type ScheduleLane } from '../model/types';

type WorkspaceScheduleDockProps = {
    lanes: ScheduleLane[];
    focusAt: DateTime;
    selectedOrderId: TypeOrNull<string>;
    selectedEngineerId: TypeOrNull<string>;
    onSelectOrder: (id: string) => void;
    onSelectEngineer: (id: TypeOrNull<string>) => void;
    fillHeight?: boolean;
    highlightOrderIds?: string[];
};

export const WorkspaceScheduleDock = ({
    lanes,
    focusAt,
    selectedOrderId,
    selectedEngineerId,
    onSelectOrder,
    onSelectEngineer,
    fillHeight = false,
    highlightOrderIds,
}: WorkspaceScheduleDockProps) => (
    <div
        className={
            fillHeight
                ? 'h-full min-h-0 overflow-hidden border-t border-border bg-white'
                : 'shrink-0 overflow-hidden border-t border-border bg-white'
        }
        aria-label="Расписание бригад"
    >
        <div
            className={
                fillHeight
                    ? 'h-full min-h-0 bg-white'
                    : 'overflow-visible bg-white'
            }
        >
            <ScheduleBoard
                lanes={lanes}
                focusAt={focusAt}
                selectedOrderId={selectedOrderId}
                selectedEngineerId={selectedEngineerId}
                onSelectOrder={onSelectOrder}
                onSelectEngineer={onSelectEngineer}
                fillHeight={fillHeight}
                highlightOrderIds={highlightOrderIds}
            />
        </div>
    </div>
);
