import { type Order, type PlanChange } from 'shared/api';
import { changeReasonLabel, workTypeLabel } from 'shared/lib/config';
import { cn, displayEngineer } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { workspaceCopy } from '../lib/config';

type ChangesPanelProps = {
    changes: PlanChange[];
    orders: Order[];
    onSelect: (orderId: string) => void;
};

export const ChangesPanel = ({
    changes,
    orders,
    onSelect,
}: ChangesPanelProps) => {
    if (!changes.length) {
        return null;
    }

    const orderById = new Map(orders.map((order) => [order.id, order]));

    return (
        <div
            className="pointer-events-auto rounded-[22px] bg-card px-4 py-3"
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <p className="text-[11px] font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                {workspaceCopy.changesTitle}
            </p>
            <div className="mt-2 flex flex-col gap-1">
                {changes.map((change) => {
                    const order = orderById.get(change.order_id);
                    const title = order
                        ? workTypeLabel[order.work_type]
                        : change.order_id;
                    const engineerId =
                        change.after?.engineer_id ?? change.before?.engineer_id;
                    const handleClick = () => {
                        onSelect(change.order_id);
                    };

                    return (
                        <Button
                            key={`${change.order_id}-${change.reason}`}
                            type="button"
                            variant="ghost"
                            className={cn(
                                'h-auto w-full justify-start rounded-2xl px-2 py-1.5',
                                'text-left text-xs whitespace-normal hover:bg-muted'
                            )}
                            onClick={handleClick}
                        >
                            <strong className="font-semibold">{title}</strong>
                            <span className="text-muted-foreground">
                                {' · '}
                                {changeReasonLabel[change.reason] ??
                                    change.reason}
                                {engineerId
                                    ? ` · ${displayEngineer(engineerId)}`
                                    : ''}
                            </span>
                        </Button>
                    );
                })}
            </div>
        </div>
    );
};
