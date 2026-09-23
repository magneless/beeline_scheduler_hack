import {
    type ComponentProps,
    createContext,
    type HTMLAttributes,
    useContext,
    useId,
} from 'react';
import {
    Controller,
    type ControllerProps,
    type FieldPath,
    type FieldValues,
    FormProvider,
    useFormContext,
} from 'react-hook-form';
import { Slot } from '@radix-ui/react-slot';

import { cn } from 'shared/lib/utils';
import { Label } from 'shared/ui/label';

const Form = FormProvider;

type FormFieldContextValue<
    TFieldValues extends FieldValues = FieldValues,
    TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = {
    name: TName;
};

const FormFieldContext = createContext<FormFieldContextValue>(
    {} as FormFieldContextValue
);

const FormField = <
    TFieldValues extends FieldValues = FieldValues,
    TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
    ...props
}: ControllerProps<TFieldValues, TName>) => (
    <FormFieldContext.Provider value={{ name: props.name }}>
        <Controller {...props} />
    </FormFieldContext.Provider>
);

const useFormField = () => {
    const fieldContext = useContext(FormFieldContext);
    const itemContext = useContext(FormItemContext);
    const { getFieldState, formState } = useFormContext();
    const fieldState = getFieldState(fieldContext.name, formState);

    return {
        id: itemContext.id,
        name: fieldContext.name,
        formItemId: `${itemContext.id}-form-item`,
        formDescriptionId: `${itemContext.id}-form-item-description`,
        formMessageId: `${itemContext.id}-form-item-message`,
        ...fieldState,
    };
};

type FormItemContextValue = {
    id: string;
};

const FormItemContext = createContext<FormItemContextValue>(
    {} as FormItemContextValue
);

const FormItem = ({ className, ...props }: HTMLAttributes<HTMLDivElement>) => {
    const id = useId();

    return (
        <FormItemContext.Provider value={{ id }}>
            <div
                data-slot="form-item"
                className={cn('space-y-1.5', className)}
                {...props}
            />
        </FormItemContext.Provider>
    );
};

const FormLabel = ({ className, ...props }: ComponentProps<typeof Label>) => {
    const { formItemId } = useFormField();

    return <Label className={className} htmlFor={formItemId} {...props} />;
};

const FormControl = ({ ...props }: ComponentProps<typeof Slot>) => {
    const { error, formItemId, formDescriptionId, formMessageId } =
        useFormField();

    return (
        <Slot
            id={formItemId}
            aria-describedby={
                error
                    ? `${formDescriptionId} ${formMessageId}`
                    : formDescriptionId
            }
            aria-invalid={Boolean(error)}
            {...props}
        />
    );
};

const FormMessage = ({
    className,
    children,
    ...props
}: HTMLAttributes<HTMLParagraphElement>) => {
    const { error, formMessageId } = useFormField();
    const body = error ? String(error.message) : children;

    if (!body) {
        return null;
    }

    return (
        <p
            id={formMessageId}
            className={cn(
                'text-[11px] font-semibold text-destructive',
                className
            )}
            {...props}
        >
            {body}
        </p>
    );
};

export { Form, FormControl, FormField, FormItem, FormLabel, FormMessage };
