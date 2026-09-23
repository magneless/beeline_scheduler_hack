import { Clock } from 'lucide-react';

import { cn } from 'shared/lib/utils';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from 'shared/ui/select';

const buildTimes = () => {
    const items: string[] = [];

    for (let minutes = 6 * 60; minutes <= 22 * 60; minutes += 15) {
        const hour = String(Math.floor(minutes / 60)).padStart(2, '0');
        const minute = String(minutes % 60).padStart(2, '0');

        items.push(`${hour}:${minute}`);
    }

    return items;
};

const SHIFT_TIMES = buildTimes();

const normalizeClock = (value: string) => {
    if (SHIFT_TIMES.includes(value) || /^\d{2}:\d{2}$/.test(value)) {
        return value;
    }

    return '09:00';
};

type TimeSelectProps = {
    value: string;
    disabled?: boolean;
    className?: string;
    'aria-label'?: string;
    onChange: (value: string) => void;
};

export const TimeSelect = ({
    value,
    disabled,
    className,
    onChange,
    'aria-label': ariaLabel,
}: TimeSelectProps) => {
    const safeValue = normalizeClock(value);
    const options = SHIFT_TIMES.includes(safeValue)
        ? SHIFT_TIMES
        : [...SHIFT_TIMES, safeValue].sort();

    return (
        <Select value={safeValue} disabled={disabled} onValueChange={onChange}>
            <SelectTrigger
                size="sm"
                aria-label={ariaLabel}
                className={cn('min-w-0 w-full bg-muted', className)}
            >
                <Clock className="size-3.5 shrink-0 text-muted-foreground" />
                <SelectValue placeholder="Время" />
            </SelectTrigger>
            <SelectContent className="max-h-60">
                {options.map((item) => (
                    <SelectItem key={item} value={item}>
                        {item}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
};
