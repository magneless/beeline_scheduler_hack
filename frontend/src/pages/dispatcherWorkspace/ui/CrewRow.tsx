import { useEffect, useRef, useState } from 'react';
import { Car, ChevronDown, Footprints, Pencil } from 'lucide-react';

import {
    type Engineer,
    type Equipment,
    type Order,
} from 'shared/api/types/contracts';
import { skillLabel, transportLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import {
    cn,
    displayEngineer,
    formatClock,
    formatCount,
    formatKm,
} from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { EngineerForm } from './EngineerForm';
import { EventTimeField } from './EventTimeField';
import { workspaceCopy } from '../lib/config';
import { crewCompatibility } from '../lib/crewCompatibility';
import { routeColor } from '../lib/routeColors';

import { type EngineerPatchInput } from '../model/types';

type CrewRowProps = {
    comparisonOrder?: Order;
    asOf?: string;
    engineer: Engineer;
    active: boolean;
    jobs: number;
    distance?: number;
    baseline?: number;
    stock: { router?: number; tv_box?: number };
    timezone: string;
    date: string;
    editing: boolean;
    canEdit?: boolean;
    canEvent?: boolean;
    defaultOccurredAt: string;
    pending?: boolean;
    onSelect: (id: TypeOrNull<string>) => void;
    onEdit: () => void;
    onPatch?: (patch: EngineerPatchInput) => void;
    onUnavailable?: (engineerId: string, occurredAt: string) => void;
};

export const CrewRow = ({
    comparisonOrder,
    asOf,
    engineer,
    active,
    jobs,
    distance,
    baseline,
    stock,
    timezone,
    date,
    editing,
    canEdit,
    canEvent,
    defaultOccurredAt,
    pending,
    onSelect,
    onEdit,
    onPatch,
    onUnavailable,
}: CrewRowProps) => {
    const rowRef = useRef<HTMLDivElement>(null);
    const [occurredAt, setOccurredAt] = useState(defaultOccurredAt);

    const compatibility = comparisonOrder
        ? crewCompatibility(comparisonOrder, engineer, stock, asOf)
        : null;
    const name = displayEngineer(engineer.id);
    const TransportIcon = engineer.transport === 'car' ? Car : Footprints;
    const stockItems = (
        Object.entries(stock) as Array<[Equipment, number]>
    ).filter(([, count]) => count > 0);
    const statusLabel = engineer.reserve
        ? 'Резерв · без задач с начала дня'
        : !engineer.available
          ? workspaceCopy.crewUnavailable
          : jobs
            ? formatCount(jobs, ['задание', 'задания', 'заданий'])
            : workspaceCopy.crewFree;
    const distanceLabel =
        distance !== undefined ? ` · ${formatKm(distance)}` : '';
    const baselineLabel =
        baseline !== undefined &&
        distance !== undefined &&
        baseline !== distance
            ? ` · ${workspaceCopy.was(formatKm(baseline))}`
            : '';

    const handleSelect = () => {
        onSelect(active ? null : engineer.id);
    };

    const handleUnavailable = () => {
        onUnavailable?.(engineer.id, occurredAt);
    };

    useEffect(() => {
        if (active) {
            rowRef.current?.scrollIntoView({
                block: 'nearest',
                behavior: 'smooth',
            });
        }
    }, [active]);

    useEffect(() => {
        if (defaultOccurredAt) {
            setOccurredAt(defaultOccurredAt);
        }
    }, [defaultOccurredAt]);

    return (
        <article
            ref={rowRef}
            data-engineer-id={engineer.id}
            className={cn(
                'relative rounded-[10px] border p-3 transition-colors',
                active
                    ? 'border-primary bg-white'
                    : 'border-transparent hover:border-border hover:bg-background'
            )}
        >
            <span
                aria-hidden="true"
                className="absolute top-3 bottom-3 left-0 w-[3px] rounded-r-full"
                style={{ backgroundColor: routeColor(engineer.id) }}
            />
            <button
                type="button"
                className={cn(
                    'flex w-full items-start gap-2.5 rounded-[4px] text-left',
                    'outline-none focus-visible:ring-2 focus-visible:ring-primary'
                )}
                onClick={handleSelect}
                aria-label={`Показать маршрут: ${name}`}
                aria-pressed={active}
                aria-expanded={active}
            >
                <TransportIcon
                    className="mt-0.5 size-5 shrink-0 text-foreground"
                    strokeWidth={1.6}
                    aria-hidden="true"
                />
                <span className="min-w-0 flex-1">
                    <span className="flex items-baseline justify-between gap-2">
                        <span className="truncate text-sm font-semibold">
                            {name}
                        </span>
                        <span className="shrink-0 text-[11px] text-muted-foreground">
                            {statusLabel}
                        </span>
                    </span>
                    <span
                        className="mt-1 block text-[11px] text-muted-foreground"
                        title={baselineLabel || undefined}
                    >
                        {transportLabel[engineer.transport]}
                        {distanceLabel}
                    </span>
                </span>
            </button>
            {compatibility ? (
                <div
                    className="mt-2 space-y-1 text-[11px]"
                    data-crew-comparison={engineer.id}
                >
                    {compatibility.length ? (
                        compatibility.map((reason) => (
                            <p key={reason} className="text-destructive">
                                {reason}
                            </p>
                        ))
                    ) : (
                        <p className="text-muted-foreground">
                            Базовые условия совпадают. Нужна проверка дороги и
                            расписания.
                        </p>
                    )}
                </div>
            ) : null}
            {active ? (
                <div className="mt-3 border-t border-border pt-3">
                    {editing && onPatch ? (
                        <EngineerForm
                            key={engineer.id}
                            engineer={engineer}
                            date={date}
                            timezone={timezone}
                            pending={pending}
                            onCancel={onEdit}
                            onSave={onPatch}
                        />
                    ) : (
                        <>
                            <dl className="grid grid-cols-[72px_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs leading-relaxed">
                                <dt className="text-muted-foreground">Смена</dt>
                                <dd className="tabular-nums">
                                    {formatClock(
                                        engineer.shift.start,
                                        timezone
                                    )}
                                    –{formatClock(engineer.shift.end, timezone)}
                                </dd>
                                <dt className="text-muted-foreground">
                                    Навыки
                                </dt>
                                <dd>
                                    {engineer.skills
                                        .map(
                                            (skill) =>
                                                skillLabel[skill] ?? skill
                                        )
                                        .join(' · ') || 'Не указаны'}
                                </dd>
                                <dt className="text-muted-foreground">
                                    Остаток
                                </dt>
                                <dd>
                                    {stockItems.map(([code, count]) => (
                                        <span key={code} className="block">
                                            {code === 'router'
                                                ? 'Роутеры'
                                                : 'Приставки'}
                                            <span className="ml-2 font-medium tabular-nums">
                                                {count}
                                            </span>
                                        </span>
                                    ))}
                                    {!stockItems.length
                                        ? 'Нет оборудования'
                                        : null}
                                </dd>
                            </dl>
                            {canEdit && onPatch ? (
                                <Button
                                    size="sm"
                                    variant="ghost"
                                    className="mt-3 h-8 rounded-[6px] px-2 text-xs font-medium"
                                    disabled={pending}
                                    onClick={onEdit}
                                >
                                    <Pencil
                                        className="size-3.5"
                                        strokeWidth={1.6}
                                    />
                                    Изменить параметры
                                </Button>
                            ) : null}
                            {canEvent &&
                            onUnavailable &&
                            (engineer.available || engineer.reserve) ? (
                                <details className="group mt-3 border-t border-border pt-3">
                                    <summary
                                        className={cn(
                                            'flex cursor-pointer list-none items-center justify-between gap-2',
                                            'rounded-[4px] text-xs text-muted-foreground hover:text-foreground',
                                            'focus-visible:outline-primary [&::-webkit-details-marker]:hidden'
                                        )}
                                    >
                                        Отметить недоступность
                                        <ChevronDown className="size-3.5 transition-transform group-open:rotate-180" />
                                    </summary>
                                    <div className="mt-3 space-y-3">
                                        <EventTimeField
                                            value={occurredAt}
                                            timezone={timezone}
                                            label="Недоступна с"
                                            compact
                                            onChange={setOccurredAt}
                                        />
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            className="h-8 w-full rounded-[6px] text-xs font-medium"
                                            disabled={pending}
                                            onClick={handleUnavailable}
                                        >
                                            Подтвердить недоступность
                                        </Button>
                                    </div>
                                </details>
                            ) : null}
                        </>
                    )}
                </div>
            ) : null}
        </article>
    );
};
