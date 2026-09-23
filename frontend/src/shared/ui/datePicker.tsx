import { CalendarDays } from 'lucide-react';
import { DateTime } from 'luxon';

import { cn } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { Calendar } from 'shared/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from 'shared/ui/popover';

const parseIsoDate = (isoDate: string) => {
    const parsed = DateTime.fromISO(isoDate);

    if (parsed.isValid) {
        return parsed;
    }

    return DateTime.now();
};

const toJsDate = (isoDate: string) => {
    const parsed = parseIsoDate(isoDate);

    return new Date(parsed.year, parsed.month - 1, parsed.day);
};

const fromJsDate = (date: Date) =>
    DateTime.fromObject({
        year: date.getFullYear(),
        month: date.getMonth() + 1,
        day: date.getDate(),
    }).toFormat('yyyy-LL-dd');

type DatePickerProps = {
    value: string;
    disabled?: boolean;
    className?: string;
    'aria-label'?: string;
    onChange: (value: string) => void;
};

export const DatePicker = ({
    value,
    disabled,
    className,
    onChange,
    'aria-label': ariaLabel,
}: DatePickerProps) => {
    const label = parseIsoDate(value).setLocale('ru').toFormat('dd.LL.yyyy');

    return (
        <Popover>
            <PopoverTrigger asChild>
                <Button
                    type="button"
                    variant="outline"
                    disabled={disabled}
                    aria-label={ariaLabel}
                    className={cn(
                        'min-w-0 justify-start bg-card px-4',
                        className
                    )}
                >
                    <CalendarDays className="size-4 shrink-0" />
                    <span className="truncate">{label}</span>
                </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-auto p-0">
                <Calendar
                    mode="single"
                    defaultMonth={toJsDate(value)}
                    selected={toJsDate(value)}
                    onSelect={(date) => {
                        if (date) {
                            onChange(fromJsDate(date));
                        }
                    }}
                />
            </PopoverContent>
        </Popover>
    );
};
