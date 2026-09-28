import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Search, Upload } from 'lucide-react';
import { toast } from 'sonner';

import {
    type Engineer,
    HttpError,
    importEngineers,
    type Order,
} from 'shared/api';
import { workTypeLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';
import { cn, formatClock } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';
import { Input } from 'shared/ui/input';
import { Tabs, TabsList, TabsTrigger } from 'shared/ui/tabs';

import { CrewRow } from './CrewRow';
import { workspaceCopy } from '../lib/config';
import { useCrewPanel } from '../model/useCrewPanel';

import { type EngineerPatchInput } from '../model/types';

type CrewPanelProps = {
    comparisonOrder?: Order;
    comparisonAddress?: string;
    asOf?: string;
    engineers: Engineer[];
    scenarioId: string;
    revision: number;
    selectedEngineerId: TypeOrNull<string>;
    timezone: string;
    date: string;
    remaining?: Record<string, { router?: number; tv_box?: number }>;
    distances?: Record<string, number>;
    baselineDistances?: Record<string, number>;
    assignedCounts?: Record<string, number>;
    canEdit?: boolean;
    canEvent?: boolean;
    defaultOccurredAt: string;
    pending?: boolean;
    onSelect: (id: TypeOrNull<string>) => void;
    onPatch?: (engineerId: string, patch: EngineerPatchInput) => void;
    onUnavailable?: (engineerId: string, occurredAt: string) => void;
};

export const CrewPanel = ({
    comparisonOrder,
    comparisonAddress,
    asOf,
    engineers,
    scenarioId,
    revision,
    selectedEngineerId,
    timezone,
    date,
    remaining,
    distances,
    baselineDistances,
    assignedCounts,
    canEdit,
    canEvent,
    defaultOccurredAt,
    pending,
    onSelect,
    onPatch,
    onUnavailable,
}: CrewPanelProps) => {
    const panel = useCrewPanel({ engineers, assignedCounts, onPatch });
    const queryClient = useQueryClient();
    const importMutation = useMutation({
        mutationFn: (file: File) =>
            importEngineers({ scenarioId, file, expectedRevision: revision }),
        onSuccess: (scenario) => {
            queryClient.setQueryData(
                ['scenarios', scenarioId, 'current'],
                scenario
            );
            queryClient.invalidateQueries({ queryKey: ['plans', scenarioId] });
            toast.success('Состав бригад заменён');
        },
        onError: (error) => {
            const message =
                error instanceof HttpError &&
                error.body.code === 'STALE_VERSION'
                    ? 'Сценарий уже изменился. Обновите страницу и повторите импорт.'
                    : error instanceof Error
                      ? error.message
                      : 'Не удалось загрузить состав';
            toast.error(message);
        },
    });

    return (
        <div className="flex h-full min-h-0 flex-col">
            <div className="shrink-0 space-y-2 px-3">
                {comparisonOrder ? (
                    <div
                        className="rounded-[8px] bg-accent px-3 py-2 text-xs"
                        aria-label="Сравнение с заявкой"
                    >
                        <strong className="block">
                            Для выбранной заявки ·{' '}
                            {workTypeLabel[comparisonOrder.work_type]}
                        </strong>
                        <span
                            className="block truncate"
                            title={comparisonAddress}
                        >
                            {comparisonAddress}
                        </span>
                        <span className="text-muted-foreground">
                            {formatClock(
                                comparisonOrder.window.start,
                                timezone
                            )}
                            –{formatClock(comparisonOrder.window.end, timezone)}
                        </span>
                    </div>
                ) : null}
                <details className="rounded-[12px] border border-border bg-white p-2 text-xs text-muted-foreground">
                    <summary className="cursor-pointer select-none font-medium text-foreground">
                        Управление бригадами
                    </summary>
                    <div className="mt-2 rounded-[8px] border border-amber-200 bg-amber-50 p-2 text-amber-900">
                        Импорт полностью заменит текущий состав бригад.
                        {!canEdit && (
                            <div className="mt-1">
                                Импорт доступен до начала событий.
                            </div>
                        )}
                        <div className="mt-2 flex items-center gap-2">
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={
                                    importMutation.isPending ||
                                    !canEdit ||
                                    pending
                                }
                                asChild
                            >
                                <label className="cursor-pointer">
                                    <Upload className="size-3.5" />
                                    {importMutation.isPending
                                        ? 'Загрузка…'
                                        : 'Загрузить CSV'}
                                    <input
                                        className="hidden"
                                        type="file"
                                        disabled={
                                            !canEdit ||
                                            pending ||
                                            importMutation.isPending
                                        }
                                        accept=".csv,text/csv"
                                        onChange={(event) => {
                                            const file =
                                                event.target.files?.[0];
                                            if (file) {
                                                importMutation.mutate(file);
                                            }
                                            event.target.value = '';
                                        }}
                                    />
                                </label>
                            </Button>
                            <a
                                className="text-xs underline"
                                href="/sample-engineers.csv"
                                download
                            >
                                Скачать шаблон
                            </a>
                        </div>
                    </div>
                </details>
                <div className="relative">
                    <Search
                        className={cn(
                            'pointer-events-none absolute top-1/2 left-3 size-3.5',
                            '-translate-y-1/2 text-muted-foreground'
                        )}
                    />
                    <Input
                        value={panel.query}
                        placeholder={workspaceCopy.crewSearch}
                        aria-label="Поиск бригад"
                        className="h-8 bg-background pl-8 text-xs"
                        onChange={panel.handleQueryChange}
                    />
                </div>
                <Tabs value={panel.load} onValueChange={panel.handleLoadChange}>
                    <TabsList
                        className={cn(
                            'grid grid-cols-2 gap-1 rounded-[12px]',
                            '[&>button]:min-w-0 [&>button]:whitespace-normal'
                        )}
                    >
                        <TabsTrigger value="all">
                            {workspaceCopy.crewFilterAll}
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {engineers.length}
                            </span>
                        </TabsTrigger>
                        <TabsTrigger value="busy">
                            {workspaceCopy.crewFilterBusy}
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {panel.busyCount}
                            </span>
                        </TabsTrigger>
                        <TabsTrigger value="free">
                            {workspaceCopy.crewFilterFree}
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {panel.freeCount}
                            </span>
                        </TabsTrigger>
                        <TabsTrigger value="reserve">
                            Резерв
                            <span className="tabular-nums text-[11px] text-muted-foreground">
                                {panel.reserveCount}
                            </span>
                        </TabsTrigger>
                    </TabsList>
                </Tabs>
            </div>
            <div className="mt-2 min-h-0 flex-1 overflow-y-auto px-2 pb-3">
                {panel.visible.length ? (
                    <div className="flex flex-col gap-1">
                        {panel.visible.map((engineer) => (
                            <CrewRow
                                key={engineer.id}
                                engineer={engineer}
                                comparisonOrder={comparisonOrder}
                                asOf={asOf}
                                active={selectedEngineerId === engineer.id}
                                jobs={assignedCounts?.[engineer.id] ?? 0}
                                distance={distances?.[engineer.id]}
                                baseline={baselineDistances?.[engineer.id]}
                                stock={
                                    remaining
                                        ? (remaining[engineer.id] ?? {})
                                        : engineer.equipment_stock
                                }
                                timezone={timezone}
                                date={date}
                                editing={panel.editingId === engineer.id}
                                canEdit={canEdit}
                                canEvent={canEvent && engineer.available}
                                defaultOccurredAt={defaultOccurredAt}
                                pending={pending}
                                onSelect={onSelect}
                                onEdit={() =>
                                    panel.handleToggleEdit(engineer.id)
                                }
                                onPatch={
                                    onPatch
                                        ? panel.handlePatch(engineer.id)
                                        : undefined
                                }
                                onUnavailable={onUnavailable}
                            />
                        ))}
                    </div>
                ) : (
                    <p className="px-3 py-10 text-center text-sm text-muted-foreground">
                        {panel.emptyMessage}
                    </p>
                )}
            </div>
        </div>
    );
};
