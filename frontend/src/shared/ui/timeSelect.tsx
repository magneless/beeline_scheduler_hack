import { Clock } from 'lucide-react';

import { cn } from 'shared/lib/utils';

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
    'aria-label': ariaLabel = 'Время',
}: TimeSelectProps) => (
    <div
        className={cn(
            'flex h-8 min-w-0 items-center gap-2 rounded-md border border-border bg-muted px-3',
            'focus-within:ring-2 focus-within:ring-primary',
            disabled && 'opacity-50',
            className
        )}
    >
        <Clock
            className="size-3.5 shrink-0 text-muted-foreground"
            aria-hidden="true"
        />
        <input
            type="time"
            step={60}
            value={value}
            disabled={disabled}
            aria-label={ariaLabel}
            className="min-w-0 w-full bg-transparent text-xs tabular-nums outline-none"
            onChange={(event) => {
                if (/^(?:[01]\d|2[0-3]):[0-5]\d$/.test(event.target.value)) {
                    onChange(event.target.value);
                }
            }}
        />
    </div>
);
