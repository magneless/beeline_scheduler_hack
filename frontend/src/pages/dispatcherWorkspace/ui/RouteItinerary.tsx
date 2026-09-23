import { useMemo } from 'react';
import { DateTime } from 'luxon';

import { type Plan, type Snapshot } from 'shared/api';
import { workTypeLabel } from 'shared/lib/config';
import { formatClock, formatCount, formatKm } from 'shared/lib/utils';

import { ScheduleBoard } from './ScheduleBoard';
import { buildSchedule } from '../lib/utils';

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
    const lanes = useMemo(
        () =>
            buildSchedule(snapshot, plan, timezone).filter(
                (lane) => lane.engineerId === engineerId
            ),
        [snapshot, plan, timezone, engineerId]
    );
    const locations = new Map(
        snapshot.locations.map((item) => [item.id, item])
    );
    const orders = new Map(snapshot.orders.map((item) => [item.id, item]));
    const distance =
        plan.metrics.per_engineer.find(
            (item) => item.engineer_id === engineerId
        )?.distance_m ?? 0;
    if (!route?.visits.length || !lanes.length) {
        return (
            <div className="shrink-0 border-t border-border bg-white px-5 py-4 text-sm text-muted-foreground">
                У этой бригады пока нет визитов в плане.
            </div>
        );
    }
    const stops = Object.fromEntries(
        route.visits.map((visit, index) => {
            const order = orders.get(visit.order_id);
            const address = order
                ? locations.get(order.location_id)?.address
                : undefined;
            return [
                visit.order_id,
                {
                    sequence: index + 1,
                    description:
                        `${index + 1} · ${order ? workTypeLabel[order.work_type] : 'Визит'} · ` +
                        `${formatClock(visit.start_at, timezone)}–${formatClock(visit.end_at, timezone)} · ` +
                        (address ?? 'Адрес не указан'),
                },
            ];
        })
    );
    return (
        <section
            aria-label="Порядок визитов"
            className="shrink-0 border-t border-border bg-white"
        >
            <ScheduleBoard
                key={engineerId}
                lanes={lanes}
                focusAt={DateTime.fromISO(route.visits[0].start_at).setZone(
                    timezone
                )}
                selectedOrderId={selectedOrderId}
                selectedEngineerId={engineerId}
                onSelectOrder={onSelect}
                onSelectEngineer={() => undefined}
                itinerary={{
                    stops,
                    departure: `Старт ${formatClock(route.start_at, timezone)}`,
                    summary:
                        formatCount(route.visits.length, [
                            'визит',
                            'визита',
                            'визитов',
                        ]) + ` · ${formatKm(distance)}`,
                }}
            />
        </section>
    );
};
