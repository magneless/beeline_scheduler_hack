import { type DemoDataset } from 'shared/api';

import { RegionCard } from './RegionCard';

type RegionCardListProps = {
    datasets: DemoDataset[];
    pending: boolean;
    onOpen: (dataset: DemoDataset) => void;
};

export const RegionCardList = ({
    datasets,
    pending,
    onOpen,
}: RegionCardListProps) => (
    <div className="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {datasets.map((dataset) => (
            <RegionCard
                key={dataset.id}
                dataset={dataset}
                disabled={pending}
                onOpen={onOpen}
            />
        ))}
    </div>
);
