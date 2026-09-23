import { type Issue, type Order, type PlanChange, type Run } from 'shared/api';
import { runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { Button } from 'shared/ui/button';

import { ChangesPanel } from './ChangesPanel';
import { IssuesBanner } from './IssuesBanner';

type WorkspaceAlertsProps = {
    issues: Issue[];
    changes: PlanChange[];
    orders: Order[];
    runStatus: TypeOrNull<Run['status']>;
    showRunBanner: boolean;
    onSelectOrder: (orderId: string) => void;
    buildErrorMessage?: string;
    onRetryBuild?: () => void;
};

export const WorkspaceAlerts = ({
    issues,
    changes,
    orders,
    runStatus,
    showRunBanner,
    onSelectOrder,
    buildErrorMessage,
    onRetryBuild,
}: WorkspaceAlertsProps) => (
    <div className="pointer-events-none absolute top-[68px] left-16 z-20 flex w-[min(320px,42%)] flex-col gap-2">
        <IssuesBanner issues={issues} onSelect={onSelectOrder} />
        <ChangesPanel
            changes={changes}
            orders={orders}
            onSelect={onSelectOrder}
        />
        {buildErrorMessage ? (
            <div
                className="pointer-events-auto rounded-[22px] bg-card px-4 py-3 text-sm"
                role="alert"
            >
                <p className="font-semibold text-destructive">
                    {buildErrorMessage}
                </p>
                {onRetryBuild ? (
                    <Button
                        className="mt-2"
                        size="sm"
                        variant="ghost"
                        onClick={onRetryBuild}
                    >
                        Повторить расчёт
                    </Button>
                ) : null}
            </div>
        ) : null}
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
