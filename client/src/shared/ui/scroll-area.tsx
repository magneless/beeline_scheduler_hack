import { type ComponentProps } from 'react';
import * as ScrollAreaPrimitive from '@radix-ui/react-scroll-area';

import { cn } from 'shared/lib/utils';

const ScrollArea = ({
    className,
    children,
    ...props
}: ComponentProps<typeof ScrollAreaPrimitive.Root>) => (
    <ScrollAreaPrimitive.Root
        className={cn('relative overflow-hidden', className)}
        {...props}
    >
        <ScrollAreaPrimitive.Viewport className="h-full w-full rounded-[inherit]">
            {children}
        </ScrollAreaPrimitive.Viewport>
        <ScrollBar />
        <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
);

const ScrollBar = ({
    className,
    orientation = 'vertical',
    ...props
}: ComponentProps<typeof ScrollAreaPrimitive.ScrollAreaScrollbar>) => (
    <ScrollAreaPrimitive.ScrollAreaScrollbar
        orientation={orientation}
        className={cn(
            'flex touch-none p-px transition-colors select-none',
            orientation === 'vertical' &&
                'h-full w-2.5 border-l border-l-transparent',
            orientation === 'horizontal' &&
                'h-2.5 flex-col border-t border-t-transparent',
            className
        )}
        {...props}
    >
        <ScrollAreaPrimitive.ScrollAreaThumb className="relative flex-1 rounded-full bg-border" />
    </ScrollAreaPrimitive.ScrollAreaScrollbar>
);

export { ScrollArea };
