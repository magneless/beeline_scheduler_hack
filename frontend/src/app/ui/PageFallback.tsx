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
        {label}
    </section>
);
