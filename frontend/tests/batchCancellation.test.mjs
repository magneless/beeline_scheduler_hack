import assert from 'node:assert/strict';
import { test } from 'node:test';

import { createEvent } from '../src/features/applyPlanEvent/model/createEvent.ts';
import { isUnassignedCancellation } from '../src/features/applyPlanEvent/model/eventRouting.ts';

const plan = {
    routes: [{ visits: [{ order_id: 'assigned' }] }],
    unassigned: [{ order_id: 'a' }, { order_id: 'b' }],
};
const snapshot = { orders: ['a', 'b', 'assigned'].map((id) => ({ id, status: 'active' })) };
const batch = {
    kind: 'cancel_many',
    orderIds: ['b', 'a'],
    occurredAt: '2026-08-17T06:00:00Z',
    reason: 'cannot_perform',
};

test('a batch carries all selected IDs in a single event without an active card', () => {
    assert.equal(isUnassignedCancellation(batch, plan, null), true);
    const event = createEvent(batch, snapshot, plan, null);
    assert.equal(event.type, 'order_cancelled');
    assert.equal(event.occurred_at, batch.occurredAt);
    assert.deepEqual(event.payload, {
        order_ids: ['b', 'a'],
        reason: 'cannot_perform',
    });
});

test('stale, duplicate, missing or empty selections cannot be submitted as a batch', () => {
    for (const orderIds of [
        [],
        ['a', 'assigned'],
        ['a', 'missing'],
        ['a', 'a'],
    ]) {
        assert.throws(() =>
            createEvent({ ...batch, orderIds }, snapshot, plan, null)
        );
    }
    assert.equal(
        isUnassignedCancellation({ ...batch, orderIds: [] }, plan, null),
        false
    );
    assert.equal(isUnassignedCancellation(batch, undefined, null), false);
});

test('individual cancellation still uses its original payload and routing rule', () => {
    const input = {
        kind: 'cancel',
        occurredAt: batch.occurredAt,
        reason: 'client_refusal',
    };
    assert.equal(isUnassignedCancellation(input, plan, 'a'), true);
    assert.equal(isUnassignedCancellation(input, plan, 'assigned'), false);
    assert.deepEqual(createEvent(input, {}, plan, 'a').payload, {
        order_id: 'a',
        reason: 'client_refusal',
    });
});

test('pending additions can be cancelled together, closed orders cannot', () => {
    const withPending = { orders: [...snapshot.orders, {id: 'pending', status: 'active'}, {id: 'closed', status: 'cancelled'}] };
    assert.deepEqual(createEvent({...batch, orderIds: ['a', 'pending']}, withPending, plan, null).payload.order_ids, ['a', 'pending']);
    assert.throws(() => createEvent({...batch, orderIds: ['closed']}, withPending, plan, null));
});
