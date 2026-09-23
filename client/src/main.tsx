import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { AppProviders } from 'app/providers/AppProviders';
import { AppRouter } from 'app/router/AppRouter';
import { env } from 'shared/config/env';

import 'app/styles/index.css';

const rootElement = document.getElementById('root');

if (!rootElement) {
    throw new Error('Root element is missing');
}

const root = createRoot(rootElement);

const renderApp = () => {
    root.render(
        <StrictMode>
            <AppProviders>
                <AppRouter />
            </AppProviders>
        </StrictMode>
    );
};

const MSW_START_TIMEOUT_MS = 2500;

const startMocks = async () => {
    const { worker } = await import('shared/api/mocks/browser');

    await Promise.race([
        worker.start({
            onUnhandledRequest: 'bypass',
            serviceWorker: { url: '/mockServiceWorker.js' },
        }),
        new Promise<void>((resolve) => {
            window.setTimeout(resolve, MSW_START_TIMEOUT_MS);
        }),
    ]);
};

const bootstrap = async () => {
    if (env.apiMode === 'mock') {
        try {
            await startMocks();
        } catch (error) {
            console.warn('MSW не стартовал, продолжаем без моков', error);
        }
    }

    renderApp();
};

void bootstrap();
