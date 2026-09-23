import { type ButtonHTMLAttributes, forwardRef } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from 'shared/lib/utils';

const buttonVariants = cva(
    `inline-flex items-center justify-center gap-2 rounded-full text-sm font-semibold
    whitespace-nowrap transition-colors outline-none
    focus-visible:ring-2 focus-visible:ring-ring
    disabled:pointer-events-none disabled:opacity-50`,
    {
        variants: {
            variant: {
                default:
                    'bg-primary text-primary-foreground hover:bg-primary/90',
                secondary:
                    'bg-foreground text-background hover:bg-foreground/85',
                outline: 'border border-border bg-card hover:bg-muted',
                ghost: 'hover:bg-muted',
                link: 'text-foreground underline-offset-4 hover:underline',
            },
            size: {
                default: 'h-11 px-5',
                sm: 'h-9 px-3 text-xs',
                lg: 'h-12 px-6 text-base',
                icon: 'size-11',
            },
        },
        defaultVariants: {
            variant: 'default',
            size: 'default',
        },
    }
);

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
    VariantProps<typeof buttonVariants> & {
        asChild?: boolean;
    };

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
    ({ className, variant, size, asChild = false, ...props }, ref) => {
        const Comp = asChild ? Slot : 'button';

        return (
            <Comp
                ref={ref}
                className={cn(buttonVariants({ variant, size, className }))}
                {...props}
            />
        );
    }
);

Button.displayName = 'Button';

export { Button, buttonVariants };
