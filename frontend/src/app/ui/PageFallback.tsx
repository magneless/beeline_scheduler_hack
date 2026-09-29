import { cn } from 'shared/lib/utils';

type PageFallbackProps = {
    label?: string;
};

export const PageFallback = ({ label = 'Загрузка…' }: PageFallbackProps) => (
    <section
        className={cn(
            'flex min-h-0 min-w-0 flex-1 items-center justify-center',
            'rounded-[32px] bg-card text-sm font-semibold text-muted-foreground'
        )}
        style={{ boxShadow: 'var(--shadow-soft)' }}
    >
        <div className="w-48" role="status">
            <p className="text-center">{label}</p>
            <div
                className="mt-3 h-2 overflow-hidden rounded-full bg-muted"
                role="progressbar"
                aria-label={label}
            >
                <div className="h-full w-1/3 animate-pulse rounded-full bg-primary" />
            </div>
        </div>
    </section>
);
