import { MapPin } from 'lucide-react';

import { type DemoDataset } from 'shared/api';
import { env } from 'shared/config/env';
import { cn, formatCount } from 'shared/lib/utils';
import { Button } from 'shared/ui/button';

import { regionMeta } from '../lib/config';

type RegionCardProps = {
    dataset: DemoDataset;
    disabled?: boolean;
    onOpen: (dataset: DemoDataset) => void;
};

export const RegionCard = ({ dataset, disabled, onOpen }: RegionCardProps) => {
    const meta = env.apiMode === 'mock' ? regionMeta[dataset.id] : undefined;

    const handleOpen = () => {
        onOpen(dataset);
    };

    return (
        <Button
            type="button"
            variant="ghost"
            disabled={disabled}
            className={cn(
                'h-auto flex-col items-stretch rounded-[28px] bg-muted p-5',
                'text-left whitespace-normal hover:bg-accent',
                'disabled:opacity-70'
            )}
            onClick={handleOpen}
        >
            <span className="flex items-start justify-between gap-3">
                <span
                    className={cn(
                        'flex size-14 items-center justify-center',
                        'rounded-full bg-primary text-2xl font-extrabold'
                    )}
                >
                    {meta
                        ? String(meta.orders).padStart(2, '0')
                        : dataset.date.slice(8)}
                </span>
                <span
                    className={cn(
                        'text-[11px] font-semibold tracking-[0.16em]',
                        'text-muted-foreground uppercase'
                    )}
                >
                    {meta?.index ?? dataset.date}
                </span>
            </span>
            <strong className="mt-5 block text-2xl font-extrabold tracking-tight">
                {dataset.name}
            </strong>
            <span className="mt-1.5 flex items-center gap-1.5 text-sm font-normal text-muted-foreground">
                <MapPin className="size-3.5 shrink-0" />
                <span className="truncate">
                    {meta?.office ?? dataset.timezone}
                </span>
            </span>
            <span className="mt-5 flex items-center justify-between gap-2">
                <span className="rounded-full bg-card px-2.5 py-1 text-[11px] font-semibold">
                    {meta
                        ? formatCount(meta.crews, [
                              'бригада',
                              'бригады',
                              'бригад',
                          ])
                        : 'Открыть сценарий'}
                </span>
                <span
                    className={cn(
                        'max-w-[18ch] text-right text-[11px] font-normal',
                        'leading-snug text-muted-foreground'
                    )}
                >
                    {meta?.tone ?? 'Синтетический набор'}
                </span>
            </span>
        </Button>
    );
};
