import { type ClassValue, clsx } from 'clsx';
import { DateTime } from 'luxon';
import { twMerge } from 'tailwind-merge';

import { engineerName } from './config';

export const cn = (...inputs: ClassValue[]) => twMerge(clsx(inputs));

export const formatClock = (iso: string, timezone: string) =>
    DateTime.fromISO(iso, { setZone: true })
        .setZone(timezone)
        .toFormat('HH:mm');

export const formatDay = (isoDate: string) =>
    DateTime.fromISO(isoDate).setLocale('ru').toFormat('d MMMM');

export const formatKm = (meters: number) => `${(meters / 1000).toFixed(1)} км`;

export const formatMinutes = (seconds: number) =>
    `${Math.round(seconds / 60)} мин`;

export const formatCount = (count: number, forms: [string, string, string]) => {
    const mod10 = count % 10;
    const mod100 = count % 100;

    if (mod10 === 1 && mod100 !== 11) {
        return `${count} ${forms[0]}`;
    }

    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) {
        return `${count} ${forms[1]}`;
    }

    return `${count} ${forms[2]}`;
};

export const displayEngineer = (id: string) => engineerName[id] ?? id;

export const toDateTimeLocal = (iso: string, timezone: string) =>
    DateTime.fromISO(iso, { setZone: true })
        .setZone(timezone)
        .toFormat("yyyy-LL-dd'T'HH:mm");

export const fromDateTimeLocal = (value: string, timezone: string) => {
    const parsed = DateTime.fromISO(value, { zone: timezone });

    if (!parsed.isValid) {
        return value;
    }

    return parsed.toUTC().toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
};

export const toClockInput = (iso: string, timezone: string) => {
    const parsed = DateTime.fromISO(iso, { setZone: true }).setZone(timezone);

    if (!parsed.isValid) {
        return DateTime.now().setZone(timezone).toFormat('HH:mm');
    }

    return parsed.toFormat('HH:mm');
};

export const fromClockInput = (
    date: string,
    clock: string,
    timezone: string
) => {
    const parsed = DateTime.fromISO(`${date}T${clock}`, { zone: timezone });

    if (!parsed.isValid) {
        return DateTime.now()
            .setZone(timezone)
            .toUTC()
            .toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
    }

    return parsed.toUTC().toFormat("yyyy-LL-dd'T'HH:mm:ss'Z'");
};
