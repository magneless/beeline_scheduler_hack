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
    <div className="shrink-0 border-t border-slate-200 bg-slate-50/60 px-4 py-2">
        {issues.length ? (
            <details className="group text-xs">
                <summary className="cursor-pointer py-2 text-slate-500 hover:text-slate-900">
                    Замечания к данным{' '}
                    <span className="ml-1 text-slate-400">{issues.length}</span>
                </summary>
                <div className="max-h-60 overflow-y-auto">
                    <IssuesBanner issues={issues} onSelect={onSelectOrder} />
                </div>
            </details>
        ) : null}
        {changes.length ? (
            <details className="text-xs">
                <summary className="cursor-pointer py-2 text-slate-500 hover:text-slate-900">
                    Последние изменения{' '}
                    <span className="ml-1 text-slate-400">
                        {changes.length}
                    </span>
                </summary>
                <div className="max-h-60 overflow-y-auto">
                    <ChangesPanel
                        changes={changes}
                        orders={orders}
                        onSelect={onSelectOrder}
                    />
                </div>
            </details>
        ) : null}
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
