import { type PendingChanges } from 'shared/api/types/contracts';
import { displayEngineer, formatClock } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

export const PendingChangesBar = ({
    queue,
    timezone,
    addresses,
    pending,
    onCalculate,
    onUndo,
}: {
    queue: PendingChanges;
    timezone: string;
    addresses: Record<string, string>;
    pending: boolean;
    onCalculate: () => void;
    onUndo: () => void;
}) => (
    <div
        className="shrink-0 border-b border-amber-200 bg-amber-50 px-5 py-3"
        role="status"
    >
        <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
                <p className="text-sm font-semibold">
                    Ожидают пересчёта: {queue.events.length}
                </p>
                <p className="text-xs text-muted-foreground">
                    На карте и в расписании предыдущий план. Изменения ещё не
                    применены к маршрутам.
                </p>
            </div>
            <Button disabled={pending} onClick={onCalculate}>
                Пересчитать маршруты
            </Button>
        </div>
        <details className="mt-2 text-xs">
            <summary className="cursor-pointer">Сохранённые изменения</summary>
            <ol className="my-2 max-h-36 space-y-1 overflow-y-auto">
                {queue.events.map((event) => {
                    const payload = event.payload;
                    const orderId = String(payload.order_id ?? '');
                    let label: string;
                    if (event.type === 'engineer_unavailable') {
                        label = `${displayEngineer(String(payload.engineer_id))} — недоступна`;
                    } else if (event.type === 'order_cancelled') {
                        const target = orderId
                            ? (addresses[orderId] ?? orderId)
                            : `${(payload.order_ids as string[]).length} заявок`;
                        label = `Отмена: ${target}`;
                    } else if (event.type === 'order_status_changed') {
                        const action =
                            payload.status === 'completed'
                                ? 'Завершение'
                                : 'Начало работы';
                        label = `${action}: ${addresses[orderId] ?? orderId}`;
                    } else {
                        const action =
                            event.type === 'urgent_order_added'
                                ? 'Новая авария'
                                : 'Новая заявка';
                        const location = payload.location as
                            { address?: string } | undefined;
                        const order = payload.order as { id: string };
                        label = `${action}: ${location?.address ?? addresses[order.id] ?? ''}`;
                    }
                    return (
                        <li key={event.id}>
                            {formatClock(event.occurred_at, timezone)} · {label}
                        </li>
                    );
                })}
            </ol>
            <Button
                size="sm"
                variant="outline"
                disabled={pending}
                onClick={onUndo}
            >
                Отменить последнее изменение
            </Button>
        </details>
    </div>
);
