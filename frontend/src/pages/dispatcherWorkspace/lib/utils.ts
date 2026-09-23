import { DateTime } from 'luxon';

import { type Plan, type Snapshot } from 'shared/api/types/contracts';
import { workTypeAppearance, workTypeLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { type MapMarker, type MapPolyline } from 'shared/ui/map';

import { crewPreviewNames } from './config';
import { routeColor } from './routeColors';

import { type ScheduleBlock, type ScheduleLane } from '../model/types';

export const assignmentOf = (
    orderId: string,
    engineerIdByOrder: Map<string, string>
) => engineerIdByOrder.get(orderId);

export const buildMapModel = (
    snapshot: Snapshot,
    plan: Plan | undefined,
    selectedEngineerId?: TypeOrNull<string>,
    onlyUnassigned = false
) => {
    const activePlan =
        plan && plan.scenario_id === snapshot.scenario_id ? plan : undefined;
    const locationById = new Map(
        snapshot.locations.map((location) => [location.id, location])
    );
    const assignedTo = new Map(
        activePlan?.routes.flatMap((route) =>
            route.visits.map(
                (visit) => [visit.order_id, route.engineer_id] as const
            )
        ) ?? []
    );
    const selectedRoute = activePlan?.routes.find(
        (route) => route.engineer_id === selectedEngineerId
    );
    const sequenceByOrder = new Map(
        selectedRoute?.visits.map((visit, index) => [
            visit.order_id,
            index + 1,
        ]) ?? []
    );
    const unassigned = new Set(
        activePlan?.unassigned.map((item) => item.order_id) ?? []
    );
    const office = locationById.get(snapshot.office_location_id);
    const markers: MapMarker[] = [];

    if (office) {
        markers.push({
            id: 'office',
            point: office.point,
            kind: 'office',
            tone: 'office',
            label: 'Офис',
        });
    }

    snapshot.orders.forEach((order) => {
        const location = locationById.get(order.location_id);

        if (
            !location ||
            order.status === 'cancelled' ||
            order.status === 'completed'
        ) {
            return;
        }
        if (onlyUnassigned && !unassigned.has(order.id)) {
            return;
        }

        if (
            selectedEngineerId &&
            assignedTo.get(order.id) !== selectedEngineerId
        ) {
            return;
        }

        markers.push({
            id: order.id,
            point: location.point,
            kind: 'order',
            open: Boolean(
                activePlan?.unassigned.some(
                    (item) => item.order_id === order.id
                )
            ),
            tone: order.work_type,
            color: selectedEngineerId
                ? workTypeAppearance[order.work_type].background
                : undefined,
            sequence: sequenceByOrder.get(order.id),
            muted: !selectedEngineerId && !unassigned.has(order.id),
            label: `${workTypeLabel[order.work_type]} · ${location.address}`,
        });
    });

    const polylines: MapPolyline[] =
        activePlan?.routes
            .filter(
                (route) =>
                    Boolean(selectedEngineerId) &&
                    route.engineer_id === selectedEngineerId
            )
            .map((route) => ({
                id: route.engineer_id,
                color: routeColor(route.engineer_id),
                points: route.legs.flatMap((leg, index) =>
                    index === 0 ? leg.geometry : leg.geometry.slice(1)
                ),
            })) ?? [];

    return { markers, polylines };
};

export const buildSchedule = (
    snapshot: Snapshot,
    plan: Plan,
    timezone: string
): ScheduleLane[] => {
    const orders = new Map(snapshot.orders.map((order) => [order.id, order]));

    return snapshot.engineers.map((engineer) => {
        const route = plan.routes.find(
            (item) => item.engineer_id === engineer.id
        );
        const start = DateTime.fromISO(engineer.shift.start, {
            setZone: true,
        }).setZone(timezone);
        const end = DateTime.fromISO(engineer.shift.end, {
            setZone: true,
        }).setZone(timezone);
        const blocks: ScheduleBlock[] = [];

        route?.visits.forEach((visit, index) => {
            const leg = route.legs[index];
            const order = orders.get(visit.order_id);
            const arrival = DateTime.fromISO(visit.arrival_at, {
                setZone: true,
            }).setZone(timezone);
            const workStart = DateTime.fromISO(visit.start_at, {
                setZone: true,
            }).setZone(timezone);
            const workEnd = DateTime.fromISO(visit.end_at, {
                setZone: true,
            }).setZone(timezone);

            if (leg) {
                blocks.push({
                    id: `${leg.id}-travel`,
                    kind: 'travel',
                    start: DateTime.fromISO(leg.start_at, {
                        setZone: true,
                    }).setZone(timezone),
                    end: DateTime.fromISO(leg.end_at, {
                        setZone: true,
                    }).setZone(timezone),
                    orderId: visit.order_id,
                });
            }

            if (workStart > arrival) {
                blocks.push({
                    id: `${visit.order_id}-wait`,
                    kind: 'wait',
                    start: arrival,
                    end: workStart,
                    orderId: visit.order_id,
                });
            }

            blocks.push({
                id: `${visit.order_id}-work`,
                kind: 'work',
                start: workStart,
                end: workEnd,
                orderId: visit.order_id,
                workType: order?.work_type,
            });
        });

        return { engineerId: engineer.id, start, end, blocks };
    });
};

export const laneIsLive = (lane: ScheduleLane, at: DateTime) =>
    lane.blocks.some(
        (block) => block.kind === 'work' && block.start <= at && block.end > at
    );

export const stretchScheduleLanes = (lanes: ScheduleLane[], count: number) => {
    if (!lanes.length || lanes.length >= count) {
        return lanes;
    }

    const extra = Array.from({ length: count - lanes.length }, (_, offset) => {
        const index = lanes.length + offset;
        const source = lanes[offset % lanes.length];
        const slide = (offset % 6) * 10;

        return {
            engineerId: `crew-${index + 1}`,
            label: crewPreviewNames[index] ?? `Бригада ${index + 1}`,
            start: source.start,
            end: source.end,
            blocks: source.blocks.map((block) => ({
                ...block,
                id: `${block.id}-${index}`,
                start: block.start.plus({ minutes: slide }),
                end: block.end.plus({ minutes: slide }),
            })),
        };
    });

    return [...lanes, ...extra];
};

export const blockOffset = (lane: ScheduleLane, block: ScheduleBlock) => {
    const total = lane.end.toMillis() - lane.start.toMillis();
    const left =
        ((block.start.toMillis() - lane.start.toMillis()) / total) * 100;
    const width =
        ((block.end.toMillis() - block.start.toMillis()) / total) * 100;

    return { left: `${left}%`, width: `${Math.max(width, 0.8)}%` };
};

export const ZOOM_DEFAULT = 240;
export const ZOOM_MIN = 56;
export const ZOOM_MAX = 720;
const ZOOM_FACTOR = 1.25;

export const clampZoom = (pxPerHour: number) =>
    Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, pxPerHour));

export const nextZoom = (pxPerHour: number, direction: 1 | -1) =>
    clampZoom(pxPerHour * (direction > 0 ? ZOOM_FACTOR : 1 / ZOOM_FACTOR));

export const fitZoom = (hourCount: number, viewWidth: number) =>
    clampZoom(viewWidth / Math.max(hourCount, 1));

export const hourTickStep = (pxPerHour: number) => {
    const hours = 72 / pxPerHour;

    if (hours <= 0.25) {
        return 0.25;
    }

    if (hours <= 0.5) {
        return 0.5;
    }

    if (hours <= 1) {
        return 1;
    }

    if (hours <= 2) {
        return 2;
    }

    return 3;
};

export const buildHourTicks = (
    start: DateTime,
    hourCount: number,
    pxPerHour: number
) => {
    const step = hourTickStep(pxPerHour);
    const ticks: Array<{ at: DateTime; left: number }> = [];

    for (let hour = 0; hour <= hourCount + 0.001; hour += step) {
        ticks.push({
            at: start.plus({ hours: hour }),
            left: hour * pxPerHour,
        });
    }

    return ticks;
};

export const isEditableTarget = (target: TypeOrNull<EventTarget>) => {
    if (!(target instanceof HTMLElement)) {
        return false;
    }

    if (target.isContentEditable) {
        return true;
    }

    const tag = target.tagName;

    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT';
};

export const focusClock = (start: DateTime, end: DateTime, now: DateTime) => {
    const mapped = start.set({
        hour: now.hour,
        minute: now.minute,
        second: now.second,
        millisecond: 0,
    });

    if (mapped <= start) {
        return start;
    }

    if (mapped >= end) {
        const noon = start.set({
            hour: 12,
            minute: 0,
            second: 0,
            millisecond: 0,
        });

        if (noon >= start && noon <= end) {
            return noon;
        }

        return start.plus({
            milliseconds: (end.toMillis() - start.toMillis()) / 2,
        });
    }

    return mapped;
};
