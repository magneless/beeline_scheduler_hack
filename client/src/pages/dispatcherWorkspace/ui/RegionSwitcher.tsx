import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
} from 'shared/ui/select';

import { workspaceCopy } from '../lib/config';
import { useRegionSwitcher } from '../model/useRegionSwitcher';

type RegionSwitcherProps = {
    currentRegionId?: string;
};

export const RegionSwitcher = ({ currentRegionId }: RegionSwitcherProps) => {
    const { datasets, pending, switchRegion } =
        useRegionSwitcher(currentRegionId);

    return (
        <Select
            value={currentRegionId}
            disabled={pending}
            onValueChange={switchRegion}
        >
            <SelectTrigger size="sm" aria-label={workspaceCopy.regionsLink}>
                {workspaceCopy.regionsLink}
            </SelectTrigger>
            <SelectContent align="end" className="min-w-[10rem]">
                {datasets.map((dataset) => (
                    <SelectItem key={dataset.id} value={dataset.region_id}>
                        {dataset.name}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
};
