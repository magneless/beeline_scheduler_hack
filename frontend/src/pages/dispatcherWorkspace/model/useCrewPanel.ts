import { type ChangeEvent, useMemo, useState } from 'react';

import { type Engineer } from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';
import { displayEngineer } from 'shared/lib/utils';

import { workspaceCopy } from '../lib/config';

import { type CrewLoadFilter, type EngineerPatchInput } from './types';

type UseCrewPanelParams = {
    engineers: Engineer[];
    assignedCounts?: Record<string, number>;
    onPatch?: (engineerId: string, patch: EngineerPatchInput) => void;
};

export const useCrewPanel = ({
    engineers,
    assignedCounts,
    onPatch,
}: UseCrewPanelParams) => {
    const [query, setQuery] = useState('');
    const [load, setLoad] = useState<CrewLoadFilter>('all');
    const [editingId, setEditingId] = useState<TypeOrNull<string>>(null);

    const busyCount = engineers.filter(
        (engineer) => (assignedCounts?.[engineer.id] ?? 0) > 0
    ).length;

    const freeCount = engineers.filter(
        (engineer) =>
            engineer.available && !(assignedCounts?.[engineer.id] ?? 0)
    ).length;

    const visible = useMemo(() => {
        const needle = query.trim().toLowerCase();
        const filtered = engineers.filter((engineer) => {
            const jobs = assignedCounts?.[engineer.id] ?? 0;

            if (load === 'busy' && jobs === 0) {
                return false;
            }

            if (load === 'free' && (jobs > 0 || !engineer.available)) {
                return false;
            }

            if (!needle) {
                return true;
            }

            return displayEngineer(engineer.id).toLowerCase().includes(needle);
        });

        return [...filtered].sort((left, right) => {
            const leftJobs = assignedCounts?.[left.id] ?? 0;
            const rightJobs = assignedCounts?.[right.id] ?? 0;

            if (leftJobs !== rightJobs) {
                return rightJobs - leftJobs;
            }

            return displayEngineer(left.id).localeCompare(
                displayEngineer(right.id),
                'ru'
            );
        });
    }, [assignedCounts, engineers, load, query]);

    const emptyMessage = query
        ? workspaceCopy.crewEmptyQuery
        : workspaceCopy.crewEmpty;

    const handleQueryChange = (event: ChangeEvent<HTMLInputElement>) => {
        setQuery(event.target.value);
    };

    const handleLoadChange = (value: string) => {
        setLoad(value as CrewLoadFilter);
    };

    const handleToggleEdit = (engineerId: string) => {
        setEditingId(editingId === engineerId ? null : engineerId);
    };

    const handlePatch = (engineerId: string) => (patch: EngineerPatchInput) => {
        onPatch?.(engineerId, patch);
        setEditingId(null);
    };

    return {
        query,
        load,
        editingId,
        busyCount,
        freeCount,
        visible,
        emptyMessage,
        handleQueryChange,
        handleLoadChange,
        handleToggleEdit,
        handlePatch,
    };
};
