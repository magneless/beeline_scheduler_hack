import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';

import { AppShell } from 'app/ui/AppShell';
import { PageFallback } from 'app/ui/PageFallback';

const ScenarioSetupPage = lazy(() => import('pages/scenarioSetup'));
const DispatcherWorkspacePage = lazy(() => import('pages/dispatcherWorkspace'));

export const AppRouter = () => (
    <AppShell>
        <Suspense fallback={<PageFallback />}>
            <Routes>
                <Route path="/" element={<ScenarioSetupPage />} />
                <Route
                    path="/s/:scenarioId"
                    element={<DispatcherWorkspacePage />}
                />
                <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
        </Suspense>
    </AppShell>
);
