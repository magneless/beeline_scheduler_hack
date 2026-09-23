import { type DateTime } from 'luxon';

import { type WorkType } from 'shared/api/types/contracts';
import { workTypeLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { cn } from 'shared/lib/utils';

import { workspaceCopy } from '../lib/config';
import { blockOffset } from '../lib/utils';

import { type ScheduleBlock, type ScheduleLane } from '../model/types';

import styles from './ScheduleBoard.module.scss';

const WORK_CLASS: Record<WorkType, string> = {
    emergency: styles.workemergency,
    connection: styles.workconnection,
    repair: styles.workrepair,
    additional: styles.workadditional,
};

type ScheduleBlockButtonProps = {
    lane: ScheduleLane;
    block: ScheduleBlock;
    focusAt: DateTime;
    selectedOrderId: TypeOrNull<string>;
    onSelectOrder: (id: string) => void;
};

export const ScheduleBlockButton = ({
    lane,
    block,
    focusAt,
    selectedOrderId,
    onSelectOrder,
}: ScheduleBlockButtonProps) => {
    if (block.kind === 'wait') {
        return null;
    }

    const workClass =
        block.kind === 'work' && block.workType
            ? WORK_CLASS[block.workType]
            : '';
    const stamp = block.start.toFormat('HH:mm');
    const label =
        block.kind === 'work' && block.workType
            ? `${workTypeLabel[block.workType]} ${stamp}`
            : workspaceCopy.scheduleTravel;

    const handleClick = () => {
        if (block.orderId) {
            onSelectOrder(block.orderId);
        }
    };

    return (
        <button
            type="button"
            title={label}
            className={cn(
                styles.block,
                block.kind === 'travel' ? styles.travel : '',
                workClass,
                block.end <= focusAt ? styles.past : '',
                block.orderId === selectedOrderId ? styles.selected : ''
            )}
            style={blockOffset(lane, block)}
            onClick={handleClick}
        >
            {block.kind === 'work' ? label : ''}
        </button>
    );
};
