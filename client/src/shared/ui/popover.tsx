import { type ComponentProps } from 'react';
import { Popover as PopoverPrimitive } from 'radix-ui';

import { cn } from 'shared/lib/utils';

const Popover = (props: ComponentProps<typeof PopoverPrimitive.Root>) => (
    <PopoverPrimitive.Root data-slot="popover" {...props} />
);

const PopoverTrigger = (
    props: ComponentProps<typeof PopoverPrimitive.Trigger>
) => <PopoverPrimitive.Trigger data-slot="popover-trigger" {...props} />;

const PopoverContent = ({
    className,
    align = 'center',
    sideOffset = 8,
    ...props
}: ComponentProps<typeof PopoverPrimitive.Content>) => (
    <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
            data-slot="popover-content"
            align={align}
            sideOffset={sideOffset}
            className={cn(
                'z-50 w-72 origin-(--radix-popover-content-transform-origin)',
                'rounded-3xl border border-border bg-popover p-4',
                'text-popover-foreground shadow-md outline-hidden',
                className
            )}
            {...props}
        />
    </PopoverPrimitive.Portal>
);

export { Popover, PopoverContent, PopoverTrigger };
