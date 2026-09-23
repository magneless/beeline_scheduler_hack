import { type ComponentProps } from 'react';
import * as SeparatorPrimitive from '@radix-ui/react-separator';

import { cn } from 'shared/lib/utils';

const Separator = ({
    className,
    orientation = 'horizontal',
    decorative = true,
    ...props
}: ComponentProps<typeof SeparatorPrimitive.Root>) => (
    <SeparatorPrimitive.Root
        decorative={decorative}
        orientation={orientation}
        className={cn(
            'shrink-0 bg-border',
            orientation === 'horizontal' ? 'h-px w-full' : 'h-full w-px',
            className
        )}
        {...props}
    />
);

export { Separator };
