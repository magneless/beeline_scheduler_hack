import { type ComponentProps } from 'react';
import { Label as LabelPrimitive } from 'radix-ui';

import { cn } from 'shared/lib/utils';

const Label = ({
    className,
    ...props
}: ComponentProps<typeof LabelPrimitive.Root>) => (
    <LabelPrimitive.Root
        data-slot="label"
        className={cn(
            'flex items-center gap-2 text-[11px] leading-none font-semibold',
            'text-muted-foreground select-none',
            'peer-disabled:cursor-not-allowed peer-disabled:opacity-50',
            className
        )}
        {...props}
    />
);

export { Label };
