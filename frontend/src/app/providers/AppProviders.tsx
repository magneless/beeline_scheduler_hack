import { type ReactNode, useState } from 'react';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'sonner';

import { ErrorBoundary } from './ErrorBoundary';

type AppProvidersProps = {
    children: ReactNode;
};

export const AppProviders = ({ children }: AppProvidersProps) => {
    const [queryClient] = useState(
        () =>
            new QueryClient({
                defaultOptions: {
                    queries: {
                        retry: false,
                        refetchOnWindowFocus: false,
                    },
                },
            })
    );

    return (
        <ErrorBoundary>
            <QueryClientProvider client={queryClient}>
                <BrowserRouter>
                    {children}
                    <Toaster
                        theme="light"
                        position="bottom-right"
                        closeButton
                        toastOptions={{
                            style: { pointerEvents: 'none' },
                            classNames: {
                                closeButton: 'pointer-events-auto',
                                actionButton: 'pointer-events-auto',
                                cancelButton: 'pointer-events-auto',
                            },
                        }}
                    />
                </BrowserRouter>
            </QueryClientProvider>
        </ErrorBoundary>
    );
};
