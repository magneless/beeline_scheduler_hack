import { ArrowRight, Navigation } from 'lucide-react';

import { type Plan, type Snapshot } from 'shared/api';
import { workTypeLabel } from 'shared/lib/config';
import {
    displayEngineer,
    formatClock,
    formatCount,
    formatKm,
} from 'shared/lib/utils';

import { routeColor } from '../lib/routeColors';

type Props = {
    snapshot: Snapshot;
    plan: Plan;
    engineerId: string;
    selectedOrderId: string | null;
    timezone: string;
    onSelect: (id: string) => void;
};
export const RouteItinerary = ({
    snapshot,
    plan,
    engineerId,
    selectedOrderId,
    timezone,
    onSelect,
}: Props) => {
    const route = plan.routes.find((item) => item.engineer_id === engineerId);
    const locations = new Map(
        snapshot.locations.map((item) => [item.id, item])
    );
    const orders = new Map(snapshot.orders.map((item) => [item.id, item]));
    const distance =
        plan.metrics.per_engineer.find(
            (item) => item.engineer_id === engineerId
        )?.distance_m ?? 0;
    if (!route?.visits.length) {
        return (
            <div className="shrink-0 border-t border-slate-200 bg-white px-5 py-4 text-sm text-slate-500">
                У этой бригады пока нет визитов в плане.
            </div>
        );
    }
    return (
        <section
            aria-label="Порядок визитов"
            className="shrink-0 border-t border-slate-200 bg-white px-5 py-4"
        >
            <div className="mb-3 flex items-center justify-between gap-3">
                <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
                    <Navigation
                        className="size-4"
                        style={{ color: routeColor(engineerId) }}
                    />
                    Порядок визитов{' '}
                    <span className="font-normal text-slate-400">
                        / {displayEngineer(engineerId)}
                    </span>
                </h2>
                <span className="shrink-0 text-xs text-slate-500">
                    {formatCount(route.visits.length, [
                        'визит',
                        'визита',
                        'визитов',
                    ])}{' '}
                    · {formatKm(distance)}
                </span>
            </div>
            <ol
                className="flex gap-2 overflow-x-auto pb-1"
                aria-label="Остановки маршрута"
            >
                <li className="flex w-28 shrink-0 items-center gap-2 text-xs text-slate-500">
                    <span className="rounded-[8px] bg-slate-100 px-3 py-2 font-medium">
                        Офис
                        <br />
                        <span className="font-normal">
                            {formatClock(route.start_at, timezone)}
                        </span>
                    </span>
                    <ArrowRight className="size-3.5" />
                </li>
                {route.visits.map((visit, index) => {
                    const order = orders.get(visit.order_id);
                    const address = order
                        ? locations.get(order.location_id)?.address
                        : undefined;
                    const active = selectedOrderId === visit.order_id;
                    return (
                        <li key={visit.order_id} className="w-56 shrink-0">
                            <button
                                type="button"
                                data-stop-order={visit.order_id}
                                aria-pressed={active}
                                onClick={() => onSelect(visit.order_id)}
                                className={[
                                    'h-full w-full rounded-[12px] border p-3 text-left transition-colors',
                                    active
                                        ? 'border-blue-300 bg-blue-50'
                                        : 'border-slate-200 bg-white hover:bg-slate-50',
                                ].join(' ')}
                            >
                                <span className="mb-2 flex items-center gap-2">
                                    <span
                                        className={[
                                            'flex size-6 items-center justify-center rounded-full',
                                            'text-xs font-bold text-white',
                                        ].join(' ')}
                                        style={{
                                            background: routeColor(engineerId),
                                        }}
                                    >
                                        {index + 1}
                                    </span>
                                    <strong className="text-xs text-slate-800">
                                        {formatClock(visit.start_at, timezone)}–
                                        {formatClock(visit.end_at, timezone)}
                                    </strong>
                                    <span className="ml-auto text-[10px] text-slate-500">
                                        {order
                                            ? workTypeLabel[order.work_type]
                                            : 'Визит'}
                                    </span>
                                </span>
                                <span
                                    className="line-clamp-2 text-xs leading-relaxed text-slate-600"
                                    title={address}
                                >
                                    {address?.replace(
                                        /^(?:г\.?\s*)?(?:Город\s+)?Москва,?\s*/i,
                                        ''
                                    ) ?? 'Адрес не указан'}
                                </span>
                            </button>
                        </li>
                    );
                })}
            </ol>
        </section>
    );
};
