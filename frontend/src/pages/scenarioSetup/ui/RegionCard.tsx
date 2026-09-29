import { ChevronRight } from 'lucide-react';

import { type DemoDataset } from 'shared/api';
import { Button } from 'shared/ui/button';

type Props = {
    dataset: DemoDataset;
    disabled?: boolean;
    onOpen: (dataset: DemoDataset) => void;
};

export const RegionCard = ({ dataset, disabled, onOpen }: Props) => (
    <Button
        type="button"
        variant="ghost"
        disabled={disabled}
        onClick={() => onOpen(dataset)}
        aria-label={`Открыть ${dataset.name}, ${dataset.date.split('-').reverse().join('.')}`}
        className="h-auto w-full justify-between gap-3 rounded-lg px-3 py-3 text-left whitespace-normal"
    >
        <span className="min-w-0 text-sm font-medium">{dataset.name}</span>
        <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
    </Button>
);
