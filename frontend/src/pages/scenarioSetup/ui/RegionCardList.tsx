import { type DemoDataset } from 'shared/api';

import { RegionCard } from './RegionCard';

type Props = {
    datasets: DemoDataset[];
    pending: boolean;
    onOpen: (dataset: DemoDataset) => void;
};

export const RegionCardList = ({ datasets, pending, onOpen }: Props) => {
    const dates = [...new Set(datasets.map((dataset) => dataset.date))]
        .sort()
        .reverse();
    return (
        <div className="mt-5 space-y-4">
            {dates.map((date) => (
                <section key={date} aria-label={`Наборы за ${date}`}>
                    <h3 className="border-b border-border pb-2 text-xs font-semibold text-muted-foreground">
                        <time dateTime={date}>
                            {date.split('-').reverse().join('.')}
                        </time>
                    </h3>
                    <div className="mt-1">
                        {datasets
                            .filter((dataset) => dataset.date === date)
                            .map((dataset) => (
                                <RegionCard
                                    key={dataset.id}
                                    dataset={dataset}
                                    disabled={pending}
                                    onOpen={onOpen}
                                />
                            ))}
                    </div>
                </section>
            ))}
        </div>
    );
};
