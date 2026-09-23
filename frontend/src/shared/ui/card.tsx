import { type HTMLAttributes } from 'react';

import { cn } from 'shared/lib/utils';

const Card = ({ className, ...props }: HTMLAttributes<HTMLDivElement>) => (
    <div
        className={cn(
            'rounded-3xl border border-border bg-card text-card-foreground shadow-sm',
            className
        )}
        {...props}
    />
);

const CardHeader = ({
    className,
    ...props
}: HTMLAttributes<HTMLDivElement>) => (
    <div
        className={cn('flex flex-col gap-1.5 p-5 pb-3', className)}
        {...props}
    />
);

const CardTitle = ({
    className,
    ...props
}: HTMLAttributes<HTMLHeadingElement>) => (
    <h2
        className={cn('text-lg font-bold tracking-tight', className)}
        {...props}
    />
);

const CardDescription = ({
    className,
    ...props
}: HTMLAttributes<HTMLParagraphElement>) => (
    <p className={cn('text-sm text-muted-foreground', className)} {...props} />
);

const CardContent = ({
    className,
    ...props
}: HTMLAttributes<HTMLDivElement>) => (
    <div className={cn('p-5 pt-0', className)} {...props} />
);

export { Card, CardContent, CardDescription, CardHeader, CardTitle };
