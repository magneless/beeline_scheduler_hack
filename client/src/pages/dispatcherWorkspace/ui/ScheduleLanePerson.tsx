import { type DateTime } from 'luxon';

import { cn, formatCount } from 'shared/lib/utils';

import { CrewAvatar } from './CrewAvatar';
import { workspaceCopy } from '../lib/config';
import { laneIsLive } from '../lib/utils';

import { type ScheduleLane } from '../model/types';

import styles from './ScheduleBoard.module.scss';

type ScheduleLanePersonProps = {
    lane: ScheduleLane;
    name: string;
    dense: boolean;
    active: boolean;
    focusAt: DateTime;
    onSelect: () => void;
};

export const ScheduleLanePerson = ({
    lane,
    name,
    dense,
    active,
    focusAt,
    onSelect,
}: ScheduleLanePersonProps) => {
    const jobs = lane.blocks.filter((block) => block.kind === 'work').length;
    const live = laneIsLive(lane, focusAt);

    return (
        <button
            type="button"
            className={cn(styles.person, active ? styles.personActive : '')}
            onClick={onSelect}
        >
            <CrewAvatar
                engineerId={lane.engineerId}
                name={name}
                size={dense ? 'xs' : 'sm'}
            />
            <span className={styles.personText}>
                <span className={styles.personName}>{name}</span>
                <span className={styles.personJobs}>
                    {jobs
                        ? formatCount(jobs, ['заявка', 'заявки', 'заявок'])
                        : workspaceCopy.crewFree}
                </span>
            </span>
            {live ? (
                <span className={styles.live} />
            ) : (
                <span className={styles.jobs}>{jobs || ''}</span>
            )}
        </button>
    );
};
