import { DateTime } from 'luxon';

export const defaultOccurredAt = (
    date: string,
    timezone: string,
    asOf?: string
) => {
    const nine = DateTime.fromISO(date, { zone: timezone }).set({
        hour: 9,
        minute: 0,
        second: 0,
        millisecond: 0,
    });
    const asOfTime = asOf
        ? DateTime.fromISO(asOf, { setZone: true }).setZone(timezone)
        : nine;
    const picked =
        asOfTime.isValid && asOfTime > nine.startOf('day') ? asOfTime : nine;

    return picked.toUTC().toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
};

export const defaultExpectedEndAt = (occurredAt: string, serviceSec: number) =>
    DateTime.fromISO(occurredAt, { setZone: true })
        .plus({ seconds: serviceSec })
        .toUTC()
        .toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");

// The time control and schedule display whole minutes. Selecting the planned
// minute refers to that appointment's precise boundary, including road seconds.
export const alignToPlannedMinute = (
    occurredAt: string,
    plannedAt?: string
) => {
    if (!plannedAt) {
        return occurredAt;
    }
    const entered = DateTime.fromISO(occurredAt, { setZone: true });
    const planned = DateTime.fromISO(plannedAt, { setZone: true });
    if (
        entered.isValid &&
        planned.isValid &&
        entered.toMillis() < planned.toMillis() &&
        entered.startOf('minute').toMillis() ===
            planned.startOf('minute').toMillis()
    ) {
        return planned.toUTC().toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
    }
    return occurredAt;
};
