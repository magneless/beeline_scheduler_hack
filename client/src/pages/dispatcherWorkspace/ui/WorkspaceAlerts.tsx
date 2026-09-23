import { type Issue, type Order, type PlanChange, type Run } from 'shared/api';
import { runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';

import { ChangesPanel } from './ChangesPanel';
import { IssuesBanner } from './IssuesBanner';

type WorkspaceAlertsProps = {
    issues: Issue[];
    changes: PlanChange[];
    orders: Order[];
    runStatus: TypeOrNull<Run['status']>;
    showRunBanner: boolean;
    onSelectOrder: (orderId: string) => void;
};

export const WorkspaceAlerts = ({
    issues,
    changes,
    orders,
    runStatus,
    showRunBanner,
    onSelectOrder,
}: WorkspaceAlertsProps) => (
    <div className="pointer-events-none absolute top-[68px] left-4 z-20 flex w-[min(320px,42%)] flex-col gap-2">
        <IssuesBanner issues={issues} onSelect={onSelectOrder} />
        <ChangesPanel
            changes={changes}
            orders={orders}
            onSelect={onSelectOrder}
        />
        {showRunBanner && runStatus ? (
            <div
                className="pointer-events-auto rounded-[22px] bg-card px-4 py-3 text-sm font-semibold"
                style={{ boxShadow: 'var(--shadow-soft)' }}
            >
                {runStatusLabel[runStatus]}
            </div>
        ) : null}
    </div>
);
