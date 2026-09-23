import { type HTMLAttributes } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from 'shared/lib/utils';

const badgeVariants = cva(
    'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold',
    {
        variants: {
            variant: {
                default: 'bg-primary text-primary-foreground',
                secondary: 'bg-muted text-foreground',
                outline: 'border border-border text-foreground',
                dark: 'bg-foreground text-background',
                danger: 'bg-destructive text-background',
            },
        },
        defaultVariants: {
            variant: 'default',
        },
    }
);

type BadgeProps = HTMLAttributes<HTMLSpanElement> &
    VariantProps<typeof badgeVariants>;

const Badge = ({ className, variant, ...props }: BadgeProps) => (
    <span className={cn(badgeVariants({ variant, className }))} {...props} />
);

export { Badge, badgeVariants };
