import { type ComponentProps } from 'react';
import { Tabs as TabsPrimitive } from 'radix-ui';

import { cn } from 'shared/lib/utils';

const Tabs = (props: ComponentProps<typeof TabsPrimitive.Root>) => (
    <TabsPrimitive.Root data-slot="tabs" {...props} />
);

const TabsList = ({
    className,
    ...props
}: ComponentProps<typeof TabsPrimitive.List>) => (
    <TabsPrimitive.List
        data-slot="tabs-list"
        className={cn(
            'inline-flex w-full items-center justify-center rounded-full bg-muted p-0.5',
            className
        )}
        {...props}
    />
);

const TabsTrigger = ({
    className,
    ...props
}: ComponentProps<typeof TabsPrimitive.Trigger>) => (
    <TabsPrimitive.Trigger
        data-slot="tabs-trigger"
        className={cn(
            'inline-flex flex-1 items-center justify-center gap-1 rounded-full',
            'px-2 py-1.5 text-xs font-semibold whitespace-nowrap outline-none',
            'text-muted-foreground transition-colors',
            'focus-visible:ring-2 focus-visible:ring-ring',
            'disabled:pointer-events-none disabled:opacity-50',
            'data-[state=active]:bg-card data-[state=active]:text-foreground',
            className
        )}
        {...props}
    />
);

const TabsContent = ({
    className,
    ...props
}: ComponentProps<typeof TabsPrimitive.Content>) => (
    <TabsPrimitive.Content
        data-slot="tabs-content"
        className={cn('outline-none', className)}
        {...props}
    />
);

export { Tabs, TabsContent, TabsList, TabsTrigger };
