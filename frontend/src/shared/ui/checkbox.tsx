import { type ComponentProps } from 'react';
import { CheckIcon } from 'lucide-react';
import { Checkbox as CheckboxPrimitive } from 'radix-ui';

import { cn } from 'shared/lib/utils';

const Checkbox = ({
    className,
    ...props
}: ComponentProps<typeof CheckboxPrimitive.Root>) => (
    <CheckboxPrimitive.Root
        data-slot="checkbox"
        className={cn(
            'peer size-4 shrink-0 rounded-[4px] border border-input bg-card',
            'outline-none transition-colors',
            'focus-visible:ring-2 focus-visible:ring-ring',
            'disabled:cursor-not-allowed disabled:opacity-50',
            'data-[state=checked]:border-primary data-[state=checked]:bg-primary',
            'data-[state=checked]:text-primary-foreground',
            className
        )}
        {...props}
    >
        <CheckboxPrimitive.Indicator
            data-slot="checkbox-indicator"
            className="grid place-content-center text-current"
        >
            <CheckIcon className="size-3.5" />
        </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
);

export { Checkbox };
