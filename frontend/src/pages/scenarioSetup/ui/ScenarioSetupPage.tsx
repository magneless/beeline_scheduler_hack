import { type DemoDataset } from 'shared/api';
import { cn } from 'shared/lib/utils';

import { RegionCardList } from './RegionCardList';
import { SetupHero } from './SetupHero';
import { SetupImportBar } from './SetupImportBar';
import { useScenarioSetup } from '../model/useScenarioSetup';

export const ScenarioSetupPage = () => {
    const {
        datasets,
        pending,
        isImporting,
        openRegion,
        openRegionError,
        retryOpenRegion,
        importOrders,
    } = useScenarioSetup();

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
                {pending && !isImporting ? (
                    <div
                        className={cn(
                            'mt-5 rounded-2xl border border-primary/30',
                            'bg-primary/10 px-4 py-3 text-sm text-foreground'
                        )}
                        role="status"
                    >
                        Открываем сценарий. Определяем координаты адресов —
                        первая загрузка может занять несколько минут.
                    </div>
                ) : null}
                {openRegionError ? (
                    <div
                        className={cn(
                            'mt-5 flex flex-wrap items-center justify-between gap-3',
                            'rounded-2xl border border-destructive/30',
                            'bg-destructive/10 px-4 py-3 text-sm text-foreground'
                        )}
                        role="alert"
                    >
                        <span>
                            {openRegionError instanceof Error
                                ? openRegionError.message
                                : 'Не удалось открыть сценарий.'}
                        </span>
                        <button
                            type="button"
                            className="rounded-full bg-card px-3 py-1.5 font-semibold hover:bg-accent"
                            onClick={retryOpenRegion}
                            disabled={pending}
                        >
                            Повторить
                        </button>
                    </div>
                ) : null}
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
