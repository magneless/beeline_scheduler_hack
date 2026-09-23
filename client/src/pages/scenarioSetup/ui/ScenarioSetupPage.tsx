import { type DemoDataset } from 'shared/api';
import { cn } from 'shared/lib/utils';

import { RegionCardList } from './RegionCardList';
import { SetupHero } from './SetupHero';
import { SetupImportBar } from './SetupImportBar';
import { useScenarioSetup } from '../model/useScenarioSetup';

export const ScenarioSetupPage = () => {
    const { datasets, pending, isImporting, openRegion, importOrders } =
        useScenarioSetup();

    const handleOpenRegion = (dataset: DemoDataset) => {
        openRegion(dataset.id);
    };

    return (
        <section
            className="relative min-h-0 min-w-0 flex-1 overflow-hidden rounded-[32px] bg-card"
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <div
                className={cn(
                    'pointer-events-none absolute -top-[18%] -right-[8%]',
                    'h-[42%] w-[28%] rounded-[46%] bg-primary/90'
                )}
            />
            <div
                className={cn(
                    'pointer-events-none absolute -bottom-[16%] -left-[6%]',
                    'h-[26%] w-[20%] rounded-[40%] bg-primary/35'
                )}
            />
            <div className="relative z-10 flex h-full flex-col justify-center px-10 py-10 lg:px-16">
                <SetupHero />
                <SetupImportBar
                    datasets={datasets}
                    pending={pending}
                    isImporting={isImporting}
                    onImport={importOrders}
                />
                <RegionCardList
                    datasets={datasets}
                    pending={pending}
                    onOpen={handleOpenRegion}
                />
            </div>
        </section>
    );
};

export default ScenarioSetupPage;
