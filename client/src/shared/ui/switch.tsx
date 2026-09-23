import { type ComponentProps, forwardRef } from 'react';
import { Switch as SwitchPrimitive } from 'radix-ui';

import { cn } from 'shared/lib/utils';

const Switch = forwardRef<
    HTMLButtonElement,
    ComponentProps<typeof SwitchPrimitive.Root>
>(({ className, ...props }, ref) => (
    <SwitchPrimitive.Root
        ref={ref}
        data-slot="switch"
        className={cn(
            'peer inline-flex h-5 w-9 shrink-0 items-center rounded-full',
            'bg-input outline-none transition-colors',
            'focus-visible:ring-2 focus-visible:ring-ring',
            'disabled:cursor-not-allowed disabled:opacity-50',
            'data-[state=checked]:bg-primary',
            className
        )}
        {...props}
    >
        <SwitchPrimitive.Thumb
            data-slot="switch-thumb"
            className={cn(
                'pointer-events-none block size-4 rounded-full bg-card',
                'transition-transform',
                'data-[state=checked]:translate-x-4',
                'data-[state=unchecked]:translate-x-0.5'
            )}
        />
    </SwitchPrimitive.Root>
));

Switch.displayName = 'Switch';

export { Switch };
