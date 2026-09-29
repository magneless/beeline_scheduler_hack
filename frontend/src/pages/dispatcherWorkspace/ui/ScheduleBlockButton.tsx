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
    if (block.end <= block.start) {
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
            : block.kind === 'wait'
              ? workspaceCopy.scheduleWait
              : workspaceCopy.scheduleTravel;

    const displayLabel =
        stop && block.kind === 'work' ? `${stop.sequence} · ${label}` : label;
    const seconds = Math.round(block.end.diff(block.start, 'seconds').seconds);
    const duration =
        seconds < 60
            ? `${seconds} с`
            : `${Math.floor(seconds / 60)} мин${seconds % 60 ? ` ${seconds % 60} с` : ''}`;
    const timing = `${block.start.toFormat('HH:mm:ss')}–${block.end.toFormat('HH:mm:ss')} · ${duration}`;
    const summary = stop
        ? block.kind === 'work'
            ? stop.description
            : `${label} · ${stop.description}`
        : label;
    const description = `${summary} · ${timing}`;
    const Tag = block.orderId ? 'button' : 'span';

    const handleClick = () => {
        if (block.orderId) {
            onSelectOrder(block.orderId);
        }
    };

    return (
        <Tag
            type={block.orderId ? 'button' : undefined}
            title={description}
            aria-label={description}
            aria-pressed={
                block.orderId ? block.orderId === selectedOrderId : undefined
            }
            data-stop-order={
                stop && block.kind === 'work' ? block.orderId : undefined
            }
            data-block-kind={block.kind}
            data-order-id={block.orderId}
            data-start-at={block.start.toISO()}
            data-end-at={block.end.toISO()}
            data-sequence={stop?.sequence}
            className={cn(
                styles.block,
                block.kind === 'travel' ? styles.travel : '',
                block.kind === 'wait' ? styles.wait : '',
                !block.orderId ? styles.unlinked : '',
                workClass,
                block.end <= focusAt ? styles.past : '',
                highlighted ? styles.highlighted : '',
                block.orderId === selectedOrderId ? styles.selected : ''
            )}
            style={blockOffset(lane, block)}
            onClick={block.orderId ? handleClick : undefined}
        >
            {block.kind !== 'travel' ? (
                <span className={styles.blockLabel}>{displayLabel}</span>
            ) : null}
        </Tag>
    );
};
