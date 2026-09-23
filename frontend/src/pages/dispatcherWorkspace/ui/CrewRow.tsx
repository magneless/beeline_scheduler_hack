import { useEffect, useRef, useState } from 'react';

import {
    type Engineer,
    type Equipment,
    type Order,
} from 'shared/api/types/contracts';
import { equipmentLabel, skillLabel, transportLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import {
    cn,
    displayEngineer,
    formatClock,
    formatCount,
    formatKm,
} from 'shared/lib/utils';
import { Badge } from 'shared/ui/badge';
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
    const stockItems = (
        Object.entries(stock) as Array<[Equipment, number]>
    ).filter(([, count]) => count > 0);
    const statusLabel = !engineer.available
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
                'rounded-[12px] border px-2.5 py-2',
                active
                    ? 'border-primary bg-accent'
                    : 'border-transparent hover:border-border hover:bg-background'
            )}
        >
            <Button
                type="button"
                variant="ghost"
                className={[
                    'h-auto w-full items-center justify-start gap-2.5 px-0 py-0',
                    'text-left whitespace-normal',
                ].join(' ')}
                onClick={handleSelect}
                aria-label={`Показать маршрут: ${name}`}
                aria-pressed={active}
            >
                <span
                    className={[
                        'flex size-8 shrink-0 items-center justify-center',
                        'rounded-[12px] text-xs font-bold text-white',
                    ].join(' ')}
                    style={{ backgroundColor: routeColor(engineer.id) }}
                >
                    {engineer.id.match(/(\d+)$/)?.[1] ?? name.slice(0, 1)}
                </span>
                <span className="min-w-0 flex-1">
                    <span className="flex items-center justify-between gap-2">
                        <span className="truncate text-sm font-bold">
                            {name}
                        </span>
                        <span className="shrink-0 text-[11px] font-semibold text-muted-foreground">
                            {statusLabel}
                        </span>
                    </span>
                    <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                        {transportLabel[engineer.transport]}
                        {distanceLabel}
                    </span>
                </span>
            </Button>
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
                <div className="mt-2 ml-[2.75rem] space-y-1.5">
                    <details className="rounded-[8px] border border-border bg-white px-2 py-1.5">
                        <summary
                            title={baselineLabel || undefined}
                            className="cursor-pointer text-[11px] font-medium text-muted-foreground"
                        >
                            Параметры бригады
                        </summary>
                        <p className="mt-2 text-[11px] text-muted-foreground">
                            Смена {formatClock(engineer.shift.start, timezone)}–
                            {formatClock(engineer.shift.end, timezone)}
                        </p>
                        <div className="mt-2 flex flex-wrap gap-1">
                            {engineer.skills.map((skill) => (
                                <Badge
                                    key={skill}
                                    variant="secondary"
                                    className="px-2 py-0.5 text-[10px]"
                                >
                                    {skillLabel[skill] ?? skill}
                                </Badge>
                            ))}
                            {stockItems.map(([code, count]) => (
                                <Badge
                                    key={code}
                                    variant="default"
                                    className="px-2 py-0.5 text-[10px]"
                                >
                                    {equipmentLabel[code]} {count}
                                </Badge>
                            ))}
                        </div>
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
                            <div className="space-y-2">
                                {canEdit ? (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={onEdit}
                                    >
                                        {workspaceCopy.crewEdit}
                                    </Button>
                                ) : null}
                                {canEvent && onUnavailable ? (
                                    <div className="min-w-0 space-y-2">
                                        <EventTimeField
                                            value={occurredAt}
                                            timezone={timezone}
                                            onChange={setOccurredAt}
                                        />
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            className="h-auto w-full whitespace-normal px-3 py-2 text-left"
                                            disabled={pending}
                                            onClick={handleUnavailable}
                                        >
                                            {workspaceCopy.crewMakeUnavailable}
                                        </Button>
                                    </div>
                                ) : null}
                            </div>
                        )}
                    </details>
                </div>
            ) : null}
        </article>
    );
};
