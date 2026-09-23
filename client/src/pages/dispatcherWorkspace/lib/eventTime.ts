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
    const picked = asOfTime > nine ? asOfTime : nine;

    return picked.toUTC().toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
};

export const defaultExpectedEndAt = (occurredAt: string, visitEnd?: string) => {
    if (visitEnd && visitEnd > occurredAt) {
        return visitEnd;
    }

    return DateTime.fromISO(occurredAt, { setZone: true })
        .plus({ minutes: 80 })
        .toUTC()
        .toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
};
