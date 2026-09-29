import { ChevronDown } from 'lucide-react';

import { type Snapshot } from 'shared/api/types/contracts';
import { Button } from 'shared/ui/button';

import { OrderAddressResolver } from './OrderAddressResolver';
import { WorkTypeBadge } from './WorkTypeBadge';

export const AddressPreflight = ({
    snapshot,
    pending,
    error,
    onBuild,
}: {
    snapshot: Snapshot;
    pending: boolean;
    error?: string;
    onBuild: () => void;
}) => {
    const unresolved = snapshot.unlocated_orders ?? [];
    return (
        <div
            className="flex min-h-0 min-w-0 flex-1 flex-col bg-background"
            aria-label="Проверка адресов перед построением маршрутов"
        >
            <div className="border-b border-border bg-card px-6 py-5">
                <h2 className="text-lg font-semibold">Проверка адресов</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                    Найдено: {snapshot.orders.length}. Требуют уточнения:{' '}
                    {unresolved.length}.
                </p>
            </div>
            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-6">
                {unresolved.length > 0 ? (
                    <p className="text-sm">
                        Уточните адреса или подтвердите точки на карте, чтобы
                        построить маршруты.
                    </p>
                ) : (
                    <p className="text-sm">
                        Все адреса определены. Можно рассчитать первый план.
                    </p>
                )}
                {unresolved.map((item) => (
                    <div
                        key={item.order.id}
                        className={[
                            'flex flex-wrap items-center justify-between gap-3',
                            'rounded-[12px] border border-border bg-card p-4',
                        ].join(' ')}
                    >
                        <div className="min-w-0 flex-1 space-y-2">
                            <WorkTypeBadge type={item.order.work_type} />
                            <p className="text-sm font-medium break-words">
                                {item.address}
                            </p>
                            <p className="text-xs text-muted-foreground">
                                {item.message}
                            </p>
                        </div>
                        <OrderAddressResolver
                            order={item.order}
                            address={item.address}
                            occurredAt={item.order.received_at}
                            pending={pending}
                        />
                    </div>
                ))}
                <details className="group rounded-[12px] border border-border bg-card p-4 text-sm">
                    <summary
                        className={[
                            'flex cursor-pointer list-none items-center justify-between gap-3 rounded-[6px]',
                            'font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring',
                            'focus-visible:ring-offset-4 [&::-webkit-details-marker]:hidden',
                        ].join(' ')}
                    >
                        <span>Найденные адреса ({snapshot.orders.length})</span>
                        <ChevronDown
                            className="size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180"
                            aria-hidden="true"
                        />
                    </summary>
                    <ul className="mt-3 space-y-2">
                        {snapshot.orders.map((order) => (
                            <li key={order.id} className="break-words">
                                {
                                    snapshot.locations.find(
                                        (location) =>
                                            location.id === order.location_id
                                    )?.address
                                }
                            </li>
                        ))}
                    </ul>
                </details>
            </div>
            <div className="space-y-2 border-t border-border bg-card p-5">
                {error ? (
                    <p role="alert" className="text-sm text-destructive">
                        {error}
                    </p>
                ) : null}
                <Button
                    disabled={
                        pending ||
                        unresolved.length > 0 ||
                        snapshot.engineers.length === 0
                    }
                    onClick={onBuild}
                >
                    {pending
                        ? 'Рассчитываем варианты…'
                        : 'Рассчитать первый план'}
                </Button>
            </div>
        </div>
    );
};
