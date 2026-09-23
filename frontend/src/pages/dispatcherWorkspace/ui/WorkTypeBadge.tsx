import { Plug, Plus, Wrench, Zap } from 'lucide-react';

import { type WorkType } from 'shared/api/types/contracts';
import { workTypeAppearance, workTypeLabel } from 'shared/lib/config';

const icons = {
    connection: Plug,
    repair: Wrench,
    emergency: Zap,
    additional: Plus,
};

export const WorkTypeBadge = ({ type }: { type: WorkType }) => {
    const Icon = icons[type];
    const appearance = workTypeAppearance[type];
    return (
        <span
            data-work-type={type}
            className="inline-flex shrink-0 items-center gap-1 rounded-[5px] px-1.5 py-1 text-[10px] font-semibold"
            style={{
                background: appearance.background,
                color: appearance.foreground,
            }}
        >
            <Icon className="size-3" aria-hidden="true" />
            {workTypeLabel[type]}
        </span>
    );
};
