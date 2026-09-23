import { type ComponentProps, forwardRef } from 'react';

import { cn } from 'shared/lib/utils';

const Input = forwardRef<HTMLInputElement, ComponentProps<'input'>>(
    ({ className, type, ...props }, ref) => (
        <input
            ref={ref}
            type={type}
            data-slot="input"
            className={cn(
                'h-11 w-full min-w-0 rounded-full border-0 bg-card px-4 text-sm',
                'outline-none transition-colors',
                'placeholder:text-muted-foreground',
                'focus-visible:ring-2 focus-visible:ring-ring',
                'disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
                'file:mr-2 file:inline-flex file:border-0 file:bg-transparent',
                'file:text-sm file:font-semibold',
                className
            )}
            {...props}
        />
    )
);

Input.displayName = 'Input';

export { Input };
