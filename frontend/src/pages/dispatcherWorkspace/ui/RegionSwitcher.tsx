import { regionLabel } from 'shared/lib/config';
import { formatCount } from 'shared/lib/utils';
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectLabel,
    SelectTrigger,
} from 'shared/ui/select';

import { workspaceCopy } from '../lib/config';
import { useRegionSwitcher } from '../model/useRegionSwitcher';

type RegionSwitcherProps = {
    currentScenarioId?: string;
};

export const RegionSwitcher = ({ currentScenarioId }: RegionSwitcherProps) => {
    const { scenarios, pending, error, refresh, switchRegion } =
        useRegionSwitcher(currentScenarioId);
    const copies = new Map<string, number>();
    for (const scenario of scenarios) {
        const key = `${scenario.region_id}:${scenario.date}`;
        copies.set(key, (copies.get(key) ?? 0) + 1);
    }

    return (
        <Select
            value={currentScenarioId ?? ''}
            disabled={!currentScenarioId}
            onValueChange={switchRegion}
            onOpenChange={(open) => {
                if (open) {
                    refresh();
                }
            }}
        >
            <SelectTrigger
                size="sm"
                aria-label={workspaceCopy.regionsLink}
                className="rounded-[8px] border border-border font-medium hover:bg-muted data-[size=sm]:h-9"
            >
                {workspaceCopy.regionsLink}
            </SelectTrigger>
            <SelectContent
                align="end"
                className="w-80 max-w-[calc(100vw-32px)]"
            >
                <SelectGroup>
                    <SelectLabel>Загруженные районы</SelectLabel>
                    {pending ? (
                        <p
                            role="status"
                            className="px-2 py-2 text-xs text-muted-foreground"
                        >
                            Загрузка списка…
                        </p>
                    ) : error ? (
                        <p
                            role="alert"
                            className="px-2 py-2 text-xs text-destructive"
                        >
                            Не удалось загрузить районы
                        </p>
                    ) : scenarios.length === 0 ? (
                        <p className="px-2 py-2 text-xs text-muted-foreground">
                            Загруженных районов пока нет
                        </p>
                    ) : null}
                    {scenarios.map((scenario) => {
                        const duplicate =
                            (copies.get(
                                `${scenario.region_id}:${scenario.date}`
                            ) ?? 0) > 1;
                        return (
                            <SelectItem
                                key={scenario.scenario_id}
                                value={scenario.scenario_id}
                            >
                                <span className="block text-xs font-medium">
                                    {regionLabel[scenario.region_id] ??
                                        scenario.region_id}
                                    {' · '}
                                    {scenario.date
                                        .split('-')
                                        .reverse()
                                        .join('.')}
                                    {duplicate ? (
                                        <span className="ml-2 font-normal text-muted-foreground">
                                            #
                                            {scenario.scenario_id
                                                .replace(/^scenario-/, '')
                                                .slice(0, 6)}
                                        </span>
                                    ) : null}
                                </span>
                                <span className="mt-1 block text-[11px] text-muted-foreground">
                                    {formatCount(scenario.order_count, [
                                        'заявка',
                                        'заявки',
                                        'заявок',
                                    ])}
                                    {' · '}
                                    {scenario.unlocated_count
                                        ? `Уточнить адреса: ${scenario.unlocated_count}`
                                        : scenario.current_plan_id
                                          ? 'План сохранён'
                                          : 'Адреса определены'}
                                </span>
                            </SelectItem>
                        );
                    })}
                </SelectGroup>
            </SelectContent>
        </Select>
    );
};
