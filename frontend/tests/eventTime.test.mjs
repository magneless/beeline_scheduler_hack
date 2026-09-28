import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
    alignToPlannedMinute,
    defaultExpectedEndAt,
} from '../src/pages/dispatcherWorkspace/lib/eventTime.ts';

test('the displayed plan minute retains road seconds at start and completion', () => {
    assert.equal(
        alignToPlannedMinute(
            '2026-08-17T12:24:00+03:00',
            '2026-08-17T09:24:03Z'
        ),
        '2026-08-17T09:24:03Z'
    );
    assert.equal(
        alignToPlannedMinute(
            '2026-08-17T13:44:00+03:00',
            '2026-08-17T10:44:03Z'
        ),
        '2026-08-17T10:44:03Z'
    );
});

test('an earlier minute or a different day never moves to the appointment', () => {
    for (const input of ['2026-08-17T09:23:00Z', '2026-08-16T09:24:00Z']) {
        assert.equal(
            alignToPlannedMinute(input, '2026-08-17T09:24:03Z'),
            input
        );
    }
});

test('the scenario clock can have hidden seconds but must never go backwards', () => {
    assert.equal(
        alignToPlannedMinute('2026-08-17T09:24:02Z', '2026-08-17T09:24:03Z'),
        '2026-08-17T09:24:03Z'
    );
    const later = '2026-08-17T09:24:45Z';
    assert.equal(alignToPlannedMinute(later, '2026-08-17T09:24:03Z'), later);
});

test('automatic completion forecast uses the precise start and service duration', () => {
    const start = alignToPlannedMinute(
        '2026-08-17T12:24:00+03:00',
        '2026-08-17T09:24:03Z'
    );
    assert.equal(defaultExpectedEndAt(start, 4800), '2026-08-17T10:44:03Z');
});

test('missing or invalid planned timestamps do not change the event', () => {
    const input = '2026-08-17T09:24:00Z';
    assert.equal(alignToPlannedMinute(input), input);
    assert.equal(alignToPlannedMinute(input, 'invalid'), input);
});
