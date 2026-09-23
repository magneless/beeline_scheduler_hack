import { cn } from 'shared/lib/utils';

import { type AssignmentFactor } from '../lib/assignmentFactors';

type AssignmentFactorsProps = {
    factors: AssignmentFactor[];
};

export const AssignmentFactors = ({ factors }: AssignmentFactorsProps) => (
    <ul className="space-y-1">
        {factors.map((factor) => (
            <li
                key={factor.id}
                className={cn(
                    'flex items-start justify-between gap-2 rounded-2xl px-3 py-1.5',
                    'bg-card text-[11px]'
                )}
            >
                <span>
                    <strong className="font-semibold">{factor.label}</strong>
                    <span className="mt-0.5 block text-muted-foreground">
                        {factor.detail}
                    </span>
                </span>
                <span
                    className={cn(
                        'shrink-0 font-semibold',
                        factor.ok ? 'text-foreground' : 'text-destructive'
                    )}
                >
                    {factor.ok ? 'учли' : 'нет'}
                </span>
            </li>
        ))}
    </ul>
);
