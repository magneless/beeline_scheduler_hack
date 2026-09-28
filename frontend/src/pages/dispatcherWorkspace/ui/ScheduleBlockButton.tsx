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
    stop?: { sequence: number; description: string };
    lane: ScheduleLane;
    block: ScheduleBlock;
    focusAt: DateTime;
    selectedOrderId: TypeOrNull<string>;
    highlighted?: boolean;
    onSelectOrder: (id: string) => void;
};

export const ScheduleBlockButton = ({
    stop,
    lane,
    block,
    focusAt,
    selectedOrderId,
    highlighted = false,
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

    const displayLabel =
        stop && block.kind === 'work' ? `${stop.sequence} · ${label}` : label;
    const description = stop
        ? block.kind === 'work'
            ? stop.description
            : `${label} · ${stop.description}`
        : label;

    const handleClick = () => {
        if (block.orderId) {
            onSelectOrder(block.orderId);
        }
    };

    return (
        <button
            type="button"
            title={description}
            aria-label={description}
            aria-pressed={block.orderId === selectedOrderId}
            data-stop-order={
                stop && block.kind === 'work' ? block.orderId : undefined
            }
            data-block-kind={block.kind}
            data-sequence={stop?.sequence}
            className={cn(
                styles.block,
                block.kind === 'travel' ? styles.travel : '',
                workClass,
                block.end <= focusAt ? styles.past : '',
                highlighted ? styles.highlighted : '',
                block.orderId === selectedOrderId ? styles.selected : ''
            )}
            style={blockOffset(lane, block)}
            onClick={handleClick}
        >
            {block.kind === 'work' ? displayLabel : ''}
        </button>
    );
};
