import { cn } from 'shared/lib/utils';

import { setupCopy } from '../lib/config';

export const SetupHero = () => (
    <>
        <p
            className={cn(
                'flex items-center gap-2 text-xs font-semibold',
                'tracking-[0.16em] text-muted-foreground uppercase'
            )}
        >
            <img src="/beeline.png" alt="" className="size-5 object-contain" />
            {setupCopy.brand}
        </p>
        <h1
            className={cn(
                'mt-3 text-2xl leading-tight font-bold tracking-tight',
                'sm:text-3xl'
            )}
        >
            {setupCopy.title}
        </h1>
        <p className="mt-2 max-w-[70ch] text-sm leading-relaxed text-muted-foreground">
            {setupCopy.description}
        </p>
    </>
);
