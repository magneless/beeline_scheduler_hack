import { DateTime } from 'luxon';

import { fromClockInput } from 'shared/lib/utils';
import { DatePicker } from 'shared/ui/datePicker';
import { Label } from 'shared/ui/label';
import { TimeSelect } from 'shared/ui/timeSelect';

type EventTimeFieldProps = {
    value: string;
    timezone: string;
    label?: string;
    onChange: (iso: string) => void;
};

const resolveInZone = (value: string, timezone: string) => {
    const parsed = DateTime.fromISO(value, { setZone: true }).setZone(timezone);

    if (parsed.isValid) {
        return parsed;
    }

    return DateTime.now().setZone(timezone);
};

export const EventTimeField = ({
    value,
    timezone,
    label = 'Когда случилось',
    onChange,
}: EventTimeFieldProps) => {
    const safe = resolveInZone(value, timezone);
    const date = safe.toFormat('yyyy-LL-dd');
    const clock = safe.toFormat('HH:mm');

    return (
        <div className="min-w-0 space-y-1">
            <Label>{label}</Label>
            <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_7.5rem] gap-2">
                <DatePicker
                    value={date}
                    className="h-8 min-w-0 w-full px-3 text-xs"
                    onChange={(nextDate) =>
                        onChange(fromClockInput(nextDate, clock, timezone))
                    }
                />
                <TimeSelect
                    value={clock}
                    aria-label="Время"
                    className="w-full"
                    onChange={(nextClock) =>
                        onChange(fromClockInput(date, nextClock, timezone))
                    }
                />
            </div>
        </div>
    );
};
