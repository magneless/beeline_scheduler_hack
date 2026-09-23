import { Button } from 'shared/ui/button';
import { Card } from 'shared/ui/card';

import { workspaceCopy } from '../lib/config';

type BuildPlanPromptProps = {
    pending: boolean;
    statusLabel?: string;
    onBuild: () => void;
    errorMessage?: string;
};

export const BuildPlanPrompt = ({
    pending,
    statusLabel,
    onBuild,
    errorMessage,
}: BuildPlanPromptProps) => (
    <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center">
        <Card
            className="pointer-events-auto w-[min(420px,90%)] px-8 py-8 text-center shadow-none"
            style={{ boxShadow: 'var(--shadow-soft)' }}
        >
            <p className="text-sm font-medium text-muted-foreground">
                {workspaceCopy.buildGreeting}
            </p>
            <h2 className="mt-2 text-3xl leading-none font-extrabold tracking-tight">
                {workspaceCopy.buildTitle}
            </h2>
            <svg
                className="mx-auto mt-5 mb-4 h-8 w-48 text-primary"
                viewBox="0 0 192 32"
                fill="none"
                aria-hidden
            >
                <path
                    d="M4 16 C20 4, 28 28, 44 16 S68 4, 84 16 108 28, 124 16 148 4, 164 16 180 28, 188 16"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                />
            </svg>
            <p className="text-sm leading-relaxed text-muted-foreground">
                {errorMessage ? (
                    <span
                        role="alert"
                        className="mt-4 block text-sm font-medium text-destructive"
                    >
                        {errorMessage}
                    </span>
                ) : null}
                {workspaceCopy.buildDescription}
            </p>
            <Button
                className="mt-6"
                size="lg"
                disabled={pending}
                onClick={onBuild}
            >
                {pending
                    ? (statusLabel ?? workspaceCopy.buildPending)
                    : errorMessage
                      ? 'Повторить расчёт'
                      : workspaceCopy.buildAction}
            </Button>
        </Card>
    </div>
);
