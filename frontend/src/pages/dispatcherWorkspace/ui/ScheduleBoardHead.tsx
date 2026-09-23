import { type ChangeEvent } from 'react';
import { CircleHelp, Search } from 'lucide-react';

import { cn } from 'shared/lib/utils';

import { workspaceCopy } from '../lib/config';

import styles from './ScheduleBoard.module.scss';

const legendMarkClass = {
    work: styles.hintMarkWork,
    travel: styles.hintMarkTravel,
    idle: styles.hintMarkIdle,
} as const;

const colorDotClass = {
    connection: styles.hintDotConnection,
    emergency: styles.hintDotEmergency,
    repair: styles.hintDotRepair,
    additional: styles.hintDotAdditional,
} as const;

type ScheduleBoardHeadProps = {
    title?: string;
    summary?: string;
    dense: boolean;
    query: string;
    liveFilter: boolean;
    onQueryChange: (value: string) => void;
    onToggleLive: () => void;
};

export const ScheduleBoardHead = ({
    title = workspaceCopy.scheduleTitle,
    summary,
    dense,
    query,
    liveFilter,
    onQueryChange,
    onToggleLive,
}: ScheduleBoardHeadProps) => {
    const handleQueryChange = (event: ChangeEvent<HTMLInputElement>) => {
        onQueryChange(event.target.value);
    };

    return (
        <div className={styles.head}>
            <div className={styles.toolbar}>
                <p className={styles.title}>{title}</p>
                {summary ? (
                    <span className="ml-auto mr-3 text-[11px] text-muted-foreground">
                        {summary}
                    </span>
                ) : null}
                <div className={styles.hint}>
                    <button
                        type="button"
                        className={styles.hintBtn}
                        aria-label={workspaceCopy.scheduleHintAria}
                    >
                        <CircleHelp />
                    </button>
                    <div className={styles.hintPop} role="tooltip">
                        <ul className={styles.hintList}>
                            {workspaceCopy.scheduleHintLegend.map((item) => (
                                <li key={item.kind}>
                                    <span
                                        className={cn(
                                            styles.hintMark,
                                            legendMarkClass[item.kind]
                                        )}
                                        aria-hidden
                                    />
                                    {item.label}
                                </li>
                            ))}
                        </ul>
                        <ul className={styles.hintList}>
                            {workspaceCopy.scheduleHintColors.map((item) => (
                                <li key={item.tone}>
                                    <span
                                        className={cn(
                                            styles.hintDot,
                                            colorDotClass[item.tone]
                                        )}
                                        aria-hidden
                                    />
                                    {item.label}
                                </li>
                            ))}
                        </ul>
                        <p className={styles.hintKeys}>
                            {workspaceCopy.scheduleHintKeys}
                        </p>
                    </div>
                </div>
            </div>
            {dense ? (
                <div className={styles.actions}>
                    <label className={styles.search}>
                        <Search />
                        <input
                            value={query}
                            placeholder={workspaceCopy.crewSearch}
                            onChange={handleQueryChange}
                        />
                    </label>
                    <button
                        type="button"
                        className={cn(
                            styles.filter,
                            liveFilter ? styles.filterOn : ''
                        )}
                        onClick={onToggleLive}
                    >
                        {workspaceCopy.scheduleLive}
                    </button>
                </div>
            ) : null}
        </div>
    );
};
