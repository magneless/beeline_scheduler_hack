import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { registerHooks } from 'node:module';
import { test } from 'node:test';

import { DateTime } from 'luxon';

registerHooks({
    resolve(specifier, context, nextResolve) {
        // Node preserves empty imports after stripping these inline type imports.
        if (['shared/ui/map', 'shared/lib/types', '../model/types'].includes(specifier)) {
            return { url: 'data:text/javascript,export {}', shortCircuit: true };
        }
        const source = specifier.startsWith('shared/')
            ? new URL(`../src/${specifier}.ts`, import.meta.url)
            : specifier.startsWith('.') && !specifier.endsWith('.ts')
              ? new URL(`${specifier}.ts`, context.parentURL)
              : null;
        return nextResolve(source && existsSync(source) ? source.href : specifier, context);
    },
});

const { buildSchedule, blockOffset } = await import('../src/pages/dispatcherWorkspace/lib/utils.ts');
const at = (clock) => `2026-08-17T${clock}+03:00`;
const leg = (id, from, to, start, end) => ({
    id, from_location_id: from, to_location_id: to,
    start_at: at(start), end_at: at(end),
});
const visit = (id, arrival, start, end) => ({
    order_id: id, arrival_at: at(arrival), start_at: at(start), end_at: at(end),
});
const snapshot = {
    engineers: [{ id: 'crew', shift: { start: at('08:00:00'), end: at('23:00:00') } }],
    orders: [
        { id: 'a', location_id: 'house-a', work_type: 'emergency' },
        { id: 'b', location_id: 'house-b', work_type: 'emergency' },
        { id: 'c', location_id: 'house-b', work_type: 'emergency' },
        { id: 'd', location_id: 'house-d', work_type: 'repair' },
    ],
};
const plan = {
    routes: [{
        engineer_id: 'crew',
        visits: [
            visit('a', '09:27:08', '09:27:08', '10:47:08'),
            visit('b', '10:47:39', '10:47:39', '12:07:39'),
            visit('c', '12:07:39', '12:07:39', '13:27:39'),
            visit('d', '13:36:04', '14:00:00', '14:30:00'),
        ],
        legs: [
            leg('elapsed', 'office', 'intermediate', '08:00:00', '09:00:00'),
            leg('arrival-a', 'intermediate', 'house-a', '09:00:00', '09:27:08'),
            leg('arrival-b', 'house-a', 'house-b', '10:47:08', '10:47:39'),
            leg('same-house', 'house-b', 'house-b', '12:07:39', '12:07:39'),
            leg('arrival-d', 'house-b', 'house-d', '13:27:39', '13:36:04'),
        ],
    }],
};

test('preserved travel does not shift the association with visits or drop the final leg', () => {
    const [lane] = buildSchedule(snapshot, plan, 'Europe/Moscow');
    const roads = lane.blocks.filter((block) => block.kind === 'travel');
    assert.deepEqual(roads.map(({ id, orderId }) => [id, orderId]), [
        ['elapsed-travel', undefined],
        ['arrival-a-travel', 'a'],
        ['arrival-b-travel', 'b'],
        ['arrival-d-travel', 'd'],
    ]);
    for (const road of roads.filter((block) => block.orderId)) {
        const appointment = plan.routes[0].visits.find((v) => v.order_id === road.orderId);
        assert.equal(road.end.toMillis(), Date.parse(appointment.arrival_at));
    }
});

test('matching uses both destination and arrival instant, independently of array order and timezone offset', () => {
    const modified = structuredClone(plan);
    modified.routes[0].legs.unshift(leg('wrong-place', 'office', 'other-house', '09:25:00', '09:27:08'));
    modified.routes[0].legs.reverse();
    modified.routes[0].visits[0].arrival_at = '2026-08-17T06:27:08Z';
    const [lane] = buildSchedule(snapshot, modified, 'Europe/Moscow');
    assert.equal(lane.blocks.find((b) => b.id === 'arrival-a-travel').orderId, 'a');
    assert.equal(lane.blocks.find((b) => b.id === 'wrong-place-travel').orderId, undefined);
});

test('same-house visits stay adjacent without a fictional travel block', () => {
    const [lane] = buildSchedule(snapshot, plan, 'Europe/Moscow');
    assert(!lane.blocks.some((block) => block.id === 'same-house-travel'));
    const b = lane.blocks.find((block) => block.id === 'b-work');
    const c = lane.blocks.find((block) => block.id === 'c-work');
    assert.equal(b.end.toMillis(), c.start.toMillis());
});

test('waiting fills exactly the interval from arrival to work, without moving either', () => {
    const [lane] = buildSchedule(snapshot, plan, 'Europe/Moscow');
    const road = lane.blocks.find((block) => block.id === 'arrival-d-travel');
    const wait = lane.blocks.find((block) => block.id === 'd-wait');
    const work = lane.blocks.find((block) => block.id === 'd-work');
    assert.equal(wait.start.toMillis(), road.end.toMillis());
    assert.equal(wait.end.toMillis(), work.start.toMillis());
    assert.equal(wait.end.diff(wait.start, 'seconds').seconds, 1436);
});

test('31 seconds and zero seconds retain their true width instead of a minimum of 7 minutes', () => {
    const [lane] = buildSchedule(snapshot, plan, 'Europe/Moscow');
    const road = lane.blocks.find((block) => block.id === 'arrival-b-travel');
    assert.equal(parseFloat(blockOffset(lane, road).width), 31 / (15 * 3600) * 100);
    assert.equal(blockOffset(lane, { ...road, end: road.start }).width, '0%');
    const equal = { ...lane, end: lane.start };
    assert.deepEqual(blockOffset(equal, road), { left: '0%', width: '0%' });
    assert.equal(DateTime.fromISO(at('10:47:08')).toMillis(), road.start.toMillis());
});
