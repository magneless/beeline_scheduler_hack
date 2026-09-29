import { useEffect, useState } from 'react';

import { type CalculationProgress } from 'shared/api/instance/httpClient';

export type CalculationState = {
    startedAt: number;
    lastEventAt: number;
    title?: string;
    progress?: CalculationProgress;
};

type Props = {
    state: CalculationState;
    title?: string;
    className?: string;
    unitLabel?: string;
    countLabel?: string;
    initialMessage?: string;
};

export const CalculationProgressBar = ({
    state,
    title = 'Расчёт маршрутов',
    className = '',
    unitLabel = 'вариантов',
    countLabel = 'Готово',
    initialMessage = 'Запускаем расчёт…',
}: Props) => {
    const [now, setNow] = useState(Date.now());
    useEffect(() => {
        setNow(Date.now());
        const timer = window.setInterval(() => setNow(Date.now()), 1000);
        return () => window.clearInterval(timer);
    }, [state.startedAt]);

    const completed = Math.max(0, state.progress?.completed ?? 0);
    const total = Math.max(0, state.progress?.total ?? 0);
    const fraction = total > 0 ? Math.min(1, completed / total) : 0;
    const elapsed = Math.max(0, Math.floor((now - state.startedAt) / 1000));
    const elapsedLabel =
        elapsed >= 60
            ? `${Math.floor(elapsed / 60)} мин ${String(elapsed % 60).padStart(2, '0')} с`
            : `${elapsed} с`;
    const connectionQuiet = now - state.lastEventAt > 10000;
    const phase = state.progress?.message || initialMessage;
    const count = `${countLabel} ${completed} из ${total} ${unitLabel}`;
    const status = connectionQuiet
        ? 'Нет новых данных от сервера. Проверяем соединение…'
        : state.progress?.variant_label
          ? `${state.progress.variant_label} · ${phase}`
          : phase;

    return (
        <div
            className={`rounded-[10px] border border-amber-300 bg-amber-50 px-4 py-3 text-foreground ${className}`}
            role="status"
            aria-live="polite"
        >
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs">
                <strong className="font-semibold">{title}</strong>
                <span className="tabular-nums text-muted-foreground">
                    {total > 0 ? `${count} · ` : ''}
                    {elapsedLabel}
                </span>
            </div>
            <div
                className="relative mt-2 h-2 overflow-hidden rounded-full bg-amber-200"
                role="progressbar"
                aria-label={title}
                aria-valuemin={0}
                aria-valuemax={total || undefined}
                aria-valuenow={total ? completed : undefined}
                aria-valuetext={total ? count : status}
            >
                <div
                    className="h-full rounded-full bg-foreground transition-[width] duration-300"
                    style={{ width: `${fraction * 100}%` }}
                />
                {!connectionQuiet && !total ? (
                    <div className="absolute inset-y-0 left-0 w-1/3 animate-pulse rounded-full bg-foreground/25" />
                ) : null}
            </div>
            <p className="mt-2 text-xs text-muted-foreground">{status}</p>
        </div>
    );
};
