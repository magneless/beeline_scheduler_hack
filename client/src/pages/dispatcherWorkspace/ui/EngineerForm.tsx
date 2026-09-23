import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

import { type Engineer, type Transport } from 'shared/api';
import {
    engineerSkillOptions,
    skillLabel,
    transportLabel,
} from 'shared/lib/config';
import { fromClockInput, toClockInput } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
} from 'shared/ui/form';
import { Input } from 'shared/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from 'shared/ui/select';
import { Switch } from 'shared/ui/switch';
import { TimeSelect } from 'shared/ui/timeSelect';

const schema = z.object({
    skills: z.array(z.string()),
    transport: z.enum(['car', 'walk']),
    shiftStart: z.string().min(1),
    shiftEnd: z.string().min(1),
    available: z.boolean(),
    router: z.number().int().min(0),
    tv_box: z.number().int().min(0),
});

type FormValues = z.infer<typeof schema>;

type EngineerFormProps = {
    engineer: Engineer;
    date: string;
    timezone: string;
    pending?: boolean;
    onCancel: () => void;
    onSave: (patch: {
        skills: string[];
        transport: Transport;
        shift: Engineer['shift'];
        available: boolean;
        equipment_stock: Engineer['equipment_stock'];
    }) => void;
};

export const EngineerForm = ({
    engineer,
    date,
    timezone,
    pending,
    onCancel,
    onSave,
}: EngineerFormProps) => {
    const form = useForm<FormValues>({
        resolver: zodResolver(schema),
        defaultValues: {
            skills: engineer.skills,
            transport: engineer.transport,
            shiftStart: toClockInput(engineer.shift.start, timezone),
            shiftEnd: toClockInput(engineer.shift.end, timezone),
            available: engineer.available,
            router: engineer.equipment_stock.router ?? 0,
            tv_box: engineer.equipment_stock.tv_box ?? 0,
        },
    });

    return (
        <Form {...form}>
            <form
                className="space-y-3.5 rounded-2xl border border-border bg-card p-3.5"
                onSubmit={form.handleSubmit((values) => {
                    onSave({
                        skills: values.skills,
                        transport: values.transport,
                        shift: {
                            start: fromClockInput(
                                date,
                                values.shiftStart,
                                timezone
                            ),
                            end: fromClockInput(
                                date,
                                values.shiftEnd,
                                timezone
                            ),
                        },
                        available: values.available,
                        equipment_stock: {
                            router: values.router,
                            tv_box: values.tv_box,
                        },
                    });
                })}
            >
                <p className="text-[11px] leading-snug text-muted-foreground">
                    До первого события можно поправить бригаду
                </p>
                <FormField
                    control={form.control}
                    name="skills"
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>Навыки</FormLabel>
                            <div className="flex flex-wrap gap-1">
                                {engineerSkillOptions.map((skill) => {
                                    const selected =
                                        field.value.includes(skill);

                                    return (
                                        <Button
                                            key={skill}
                                            type="button"
                                            size="sm"
                                            variant={
                                                selected ? 'default' : 'outline'
                                            }
                                            className="h-7 px-2.5 text-[11px]"
                                            onClick={() => {
                                                field.onChange(
                                                    selected
                                                        ? field.value.filter(
                                                              (item) =>
                                                                  item !== skill
                                                          )
                                                        : [
                                                              ...field.value,
                                                              skill,
                                                          ]
                                                );
                                            }}
                                        >
                                            {skillLabel[skill] ?? skill}
                                        </Button>
                                    );
                                })}
                            </div>
                        </FormItem>
                    )}
                />
                <div className="grid grid-cols-2 gap-3">
                    <FormField
                        control={form.control}
                        name="transport"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>Транспорт</FormLabel>
                                <Select
                                    value={field.value}
                                    onValueChange={field.onChange}
                                >
                                    <FormControl>
                                        <SelectTrigger
                                            size="sm"
                                            className="w-full bg-muted"
                                        >
                                            <SelectValue placeholder="Транспорт" />
                                        </SelectTrigger>
                                    </FormControl>
                                    <SelectContent>
                                        <SelectItem value="car">
                                            {transportLabel.car}
                                        </SelectItem>
                                        <SelectItem value="walk">
                                            {transportLabel.walk}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </FormItem>
                        )}
                    />
                    <FormField
                        control={form.control}
                        name="available"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>На линии</FormLabel>
                                <FormControl>
                                    <div className="flex h-8 items-center">
                                        <Switch
                                            checked={field.value}
                                            onCheckedChange={field.onChange}
                                        />
                                    </div>
                                </FormControl>
                            </FormItem>
                        )}
                    />
                </div>
                <div className="grid grid-cols-2 gap-3">
                    <FormField
                        control={form.control}
                        name="shiftStart"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>Начало смены</FormLabel>
                                <TimeSelect
                                    value={field.value}
                                    aria-label="Начало смены"
                                    onChange={field.onChange}
                                />
                            </FormItem>
                        )}
                    />
                    <FormField
                        control={form.control}
                        name="shiftEnd"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>Конец смены</FormLabel>
                                <TimeSelect
                                    value={field.value}
                                    aria-label="Конец смены"
                                    onChange={field.onChange}
                                />
                            </FormItem>
                        )}
                    />
                    <FormField
                        control={form.control}
                        name="router"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>Роутеры</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        min={0}
                                        className="h-8 bg-muted px-3 text-xs"
                                        {...field}
                                        onChange={(event) =>
                                            field.onChange(
                                                event.target.valueAsNumber
                                            )
                                        }
                                    />
                                </FormControl>
                            </FormItem>
                        )}
                    />
                    <FormField
                        control={form.control}
                        name="tv_box"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>Приставки</FormLabel>
                                <FormControl>
                                    <Input
                                        type="number"
                                        min={0}
                                        className="h-8 bg-muted px-3 text-xs"
                                        {...field}
                                        onChange={(event) =>
                                            field.onChange(
                                                event.target.valueAsNumber
                                            )
                                        }
                                    />
                                </FormControl>
                            </FormItem>
                        )}
                    />
                </div>
                <div className="flex gap-2 pt-0.5">
                    <Button type="submit" size="sm" disabled={pending}>
                        {pending ? 'Сохраняем…' : 'Сохранить'}
                    </Button>
                    <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={onCancel}
                    >
                        Отмена
                    </Button>
                </div>
            </form>
        </Form>
    );
};
