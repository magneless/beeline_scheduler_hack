import { type DemoDataset } from 'shared/api';
import { Button } from 'shared/ui/button';
import { CalculationProgressBar } from 'shared/ui/calculationProgress';

import { RegionCardList } from './RegionCardList';
import { SetupHero } from './SetupHero';
import { SetupImportBar } from './SetupImportBar';
import { useScenarioSetup } from '../model/useScenarioSetup';

export const ScenarioSetupPage = () => {
    const {
        datasets,
        datasetsLoading,
        datasetsError,
        retryDatasets,
        pending,
        progress,
        isImporting,
        openRegion,
        openRegionError,
        retryOpenRegion,
        importOrders,
        importError,
        resetImportError,
    } = useScenarioSetup();
    const handleOpen = (dataset: DemoDataset) => openRegion(dataset.id);
    return (
        <section className="min-h-0 min-w-0 flex-1 overflow-hidden rounded-[24px] bg-card">
            <div
                className="h-full overflow-y-auto overscroll-contain"
                role="region"
                aria-label="Подготовка дня"
                tabIndex={0}
            >
                <div className="mx-auto max-w-[1400px] px-4 py-6 sm:px-8 sm:py-8">
                    <header>
                        <SetupHero />
                    </header>
                    {pending && progress ? (
                        <div className="sticky top-3 z-20 mt-5">
                            <CalculationProgressBar
                                state={progress}
                                title={
                                    isImporting
                                        ? 'Подготовка дня'
                                        : 'Открываем набор заявок'
                                }
                                unitLabel="адресов"
                                countLabel="Обработано"
                                initialMessage="Проверяем данные…"
                                className="shadow-sm"
                            />
                        </div>
                    ) : null}
                    <div className="mt-7 grid items-stretch gap-6 lg:grid-cols-2">
                        <aside
                            className="min-w-0 rounded-2xl border border-border p-5 sm:p-6"
                            aria-label="Готовые наборы заявок"
                        >
                            <h2 className="text-base font-bold">
                                Готовые наборы заявок
                            </h2>
                            <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
                                В наборах используются демонстрационные бригады.
                            </p>
                            {datasetsLoading ? (
                                <p
                                    role="status"
                                    className="mt-5 text-sm text-muted-foreground"
                                >
                                    Загружаем наборы…
                                </p>
                            ) : null}
                            {datasetsError ? (
                                <div className="mt-4 space-y-2" role="alert">
                                    <p className="text-sm text-destructive">
                                        Не удалось загрузить готовые наборы.
                                    </p>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        onClick={retryDatasets}
                                    >
                                        Повторить
                                    </Button>
                                </div>
                            ) : null}
                            {openRegionError ? (
                                <div className="mt-4 space-y-2" role="alert">
                                    <p className="text-sm text-destructive">
                                        {openRegionError.message}
                                    </p>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        disabled={pending}
                                        onClick={retryOpenRegion}
                                    >
                                        Повторить открытие
                                    </Button>
                                </div>
                            ) : null}
                            <RegionCardList
                                datasets={datasets}
                                pending={pending}
                                onOpen={handleOpen}
                            />
                        </aside>
                        <SetupImportBar
                            pending={pending}
                            error={importError}
                            onEdit={resetImportError}
                            onImport={importOrders}
                        />
                    </div>
                </div>
            </div>
        </section>
    );
};

export default ScenarioSetupPage;
