import { cn } from 'shared/lib/utils';

type TimeSelectProps = {
    id?: string;
    value: string;
    disabled?: boolean;
    className?: string;
    'aria-label'?: string;
    onChange: (value: string) => void;
};

export const TimeSelect = ({
    id,
    value,
    disabled,
    className,
    onChange,
    'aria-label': ariaLabel = 'Время',
}: TimeSelectProps) => (
    <div
        className={cn(
            'flex h-8 min-w-0 items-center rounded-md border border-border bg-muted px-3',
            'focus-within:ring-2 focus-within:ring-primary',
            disabled && 'opacity-50',
            className
        )}
    >
        <input
            id={id}
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
