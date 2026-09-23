import { type ComponentProps, forwardRef } from 'react';
import { CheckIcon, ChevronDownIcon, ChevronUpIcon } from 'lucide-react';
import { Select as SelectPrimitive } from 'radix-ui';

import { cn } from 'shared/lib/utils';

const Select = (props: ComponentProps<typeof SelectPrimitive.Root>) => (
    <SelectPrimitive.Root data-slot="select" {...props} />
);

const SelectGroup = (props: ComponentProps<typeof SelectPrimitive.Group>) => (
    <SelectPrimitive.Group data-slot="select-group" {...props} />
);

const SelectValue = (props: ComponentProps<typeof SelectPrimitive.Value>) => (
    <SelectPrimitive.Value data-slot="select-value" {...props} />
);

const SelectTrigger = forwardRef<
    HTMLButtonElement,
    ComponentProps<typeof SelectPrimitive.Trigger> & {
        size?: 'sm' | 'default';
    }
>(({ className, size = 'default', children, ...props }, ref) => (
    <SelectPrimitive.Trigger
        ref={ref}
        data-slot="select-trigger"
        data-size={size}
        className={cn(
            'flex w-fit items-center justify-between gap-2 rounded-full',
            'border-0 bg-card px-4 text-sm whitespace-nowrap outline-none',
            'transition-colors focus-visible:ring-2 focus-visible:ring-ring',
            'disabled:cursor-not-allowed disabled:opacity-50',
            'data-[placeholder]:text-muted-foreground data-[size=default]:h-11',
            'data-[size=sm]:h-8 data-[size=sm]:px-3 data-[size=sm]:text-xs',
            '[&_svg]:pointer-events-none [&_svg]:shrink-0',
            "[&_svg:not([class*='size-'])]:size-4",
            className
        )}
        {...props}
    >
        {children}
        <SelectPrimitive.Icon asChild>
            <ChevronDownIcon className="size-4 opacity-50" />
        </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
));

SelectTrigger.displayName = 'SelectTrigger';

const SelectScrollUpButton = ({
    className,
    ...props
}: ComponentProps<typeof SelectPrimitive.ScrollUpButton>) => (
    <SelectPrimitive.ScrollUpButton
        data-slot="select-scroll-up-button"
        className={cn(
            'flex cursor-default items-center justify-center py-1',
            className
        )}
        {...props}
    >
        <ChevronUpIcon className="size-4" />
    </SelectPrimitive.ScrollUpButton>
);

const SelectScrollDownButton = ({
    className,
    ...props
}: ComponentProps<typeof SelectPrimitive.ScrollDownButton>) => (
    <SelectPrimitive.ScrollDownButton
        data-slot="select-scroll-down-button"
        className={cn(
            'flex cursor-default items-center justify-center py-1',
            className
        )}
        {...props}
    >
        <ChevronDownIcon className="size-4" />
    </SelectPrimitive.ScrollDownButton>
);

const SelectContent = ({
    className,
    children,
    position = 'popper',
    align = 'center',
    ...props
}: ComponentProps<typeof SelectPrimitive.Content>) => (
    <SelectPrimitive.Portal>
        <SelectPrimitive.Content
            data-slot="select-content"
            className={cn(
                'relative z-50 max-h-60 min-w-[8rem]',
                'overflow-x-hidden overflow-y-auto rounded-2xl',
                'border border-border bg-popover text-popover-foreground shadow-md',
                className
            )}
            position={position}
            align={align}
            {...props}
        >
            <SelectScrollUpButton />
            <SelectPrimitive.Viewport
                className={cn(
                    'p-1',
                    position === 'popper' &&
                        'w-full min-w-[var(--radix-select-trigger-width)]'
                )}
            >
                {children}
            </SelectPrimitive.Viewport>
            <SelectScrollDownButton />
        </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
);

const SelectLabel = ({
    className,
    ...props
}: ComponentProps<typeof SelectPrimitive.Label>) => (
    <SelectPrimitive.Label
        data-slot="select-label"
        className={cn('px-2 py-1.5 text-xs text-muted-foreground', className)}
        {...props}
    />
);

const SelectItem = ({
    className,
    children,
    ...props
}: ComponentProps<typeof SelectPrimitive.Item>) => (
    <SelectPrimitive.Item
        data-slot="select-item"
        className={cn(
            'relative flex w-full cursor-default items-center gap-2 rounded-xl',
            'py-1.5 pr-8 pl-2 text-sm outline-hidden select-none',
            'focus:bg-accent focus:text-accent-foreground',
            'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
            className
        )}
        {...props}
    >
        <span className="absolute right-2 flex size-3.5 items-center justify-center">
            <SelectPrimitive.ItemIndicator>
                <CheckIcon className="size-4" />
            </SelectPrimitive.ItemIndicator>
        </span>
        <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
);

export {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectLabel,
    SelectTrigger,
    SelectValue,
};
