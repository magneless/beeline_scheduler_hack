import { cn, displayEngineer } from 'shared/lib/utils';

import { crewToneClass } from '../lib/config';

type CrewAvatarProps = {
    engineerId: string;
    name?: string;
    size?: 'xs' | 'sm' | 'md';
};

const TONE_FALLBACK = [
    'bg-primary',
    'bg-foreground',
    'bg-muted-foreground',
    'bg-accent',
] as const;

const toneOf = (engineerId: string) => {
    const base = engineerId.split('-')[0];
    const known = crewToneClass[engineerId] ?? crewToneClass[base];

    if (known) {
        return known;
    }

    const hash = [...engineerId].reduce(
        (sum, char) => sum + char.charCodeAt(0),
        0
    );

    return TONE_FALLBACK[hash % TONE_FALLBACK.length];
};

export const CrewAvatar = ({
    engineerId,
    name,
    size = 'md',
}: CrewAvatarProps) => {
    const label = name ?? displayEngineer(engineerId);
    const tone = toneOf(engineerId);
    const inverted = tone === 'bg-foreground' || tone === 'bg-muted-foreground';

    return (
        <span
            className={cn(
                'flex shrink-0 items-center justify-center font-extrabold',
                size === 'xs' && 'size-6 rounded-lg text-[10px]',
                size === 'sm' && 'size-8 rounded-xl text-xs',
                size === 'md' && 'size-9 rounded-2xl text-sm',
                tone,
                inverted ? 'text-background' : 'text-foreground'
            )}
        >
            {label.slice(0, 1)}
        </span>
    );
};
