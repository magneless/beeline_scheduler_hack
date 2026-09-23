import { Component, type ErrorInfo, type ReactNode } from 'react';

import { Button } from 'shared/ui/button';

type ErrorBoundaryProps = {
    children: ReactNode;
};

type ErrorBoundaryState = {
    hasError: boolean;
    message: string;
};

export class ErrorBoundary extends Component<
    ErrorBoundaryProps,
    ErrorBoundaryState
> {
    state: ErrorBoundaryState = {
        hasError: false,
        message: '',
    };

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return {
            hasError: true,
            message: error.message || 'Что-то пошло не так',
        };
    }

    componentDidCatch(error: Error, info: ErrorInfo) {
        console.error('ErrorBoundary caught', error, info.componentStack);
    }

    private handleReload = () => {
        window.location.assign('/');
    };

    render() {
        if (!this.state.hasError) {
            return this.props.children;
        }

        return (
            <section className="flex min-h-screen items-center justify-center bg-background px-6">
                <div
                    className="w-full max-w-md rounded-[28px] bg-card p-8 text-center"
                    style={{ boxShadow: 'var(--shadow-soft)' }}
                >
                    <p className="text-[11px] font-semibold tracking-[0.16em] text-muted-foreground uppercase">
                        Ошибка интерфейса
                    </p>
                    <h1 className="mt-3 text-2xl font-extrabold tracking-tight">
                        Смена не открылась
                    </h1>
                    <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
                        {this.state.message ||
                            'Попробуйте обновить страницу или вернуться к районам.'}
                    </p>
                    <Button className="mt-6" onClick={this.handleReload}>
                        К районам
                    </Button>
                </div>
            </section>
        );
    }
}
