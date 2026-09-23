import { useEffect, useMemo } from 'react';

import {
    type PlanEventInput,
    useApplyPlanEvent,
} from 'features/applyPlanEvent';
import { useBuildPlan } from 'features/buildPlan';
import { type CrewPatchInput, useUpdateCrew } from 'features/updateCrew';
import { type Run } from 'shared/api/types/contracts';
import { runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';

import { useWorkspaceQueries } from './queries';
import { useDispatcherWorkspaceStore } from './store';
import { useWorkspaceSelection } from './useWorkspaceSelection';
import { buildMapModel } from '../lib/utils';
import { buildWorkspaceView } from '../lib/viewModel';

export const useDispatcherWorkspace = (scenarioId: string) => {
    const selection = useWorkspaceSelection();
    const queries = useWorkspaceQueries(scenarioId);
    useEffect(() => {
        useDispatcherWorkspaceStore.getState().resetSelection();
    }, [scenarioId]);

    const mapModel = useMemo(
        () =>
            queries.snapshot
                ? buildMapModel(
                      queries.snapshot,
                      queries.plan,
                      selection.selectedEngineerId,
                      selection.panelTab === 'orders' &&
                          selection.filter === 'unassigned'
                  )
                : { markers: [], polylines: [] },
        [
            queries.snapshot,
            queries.plan,
            selection.selectedEngineerId,
            selection.panelTab,
            selection.filter,
        ]
    );

    const view = useMemo(
        () =>
            buildWorkspaceView({
                snapshot: queries.snapshot,
                plan: queries.plan,
                compareMetrics: queries.compareMetrics,
                selectedOrderId: selection.selectedOrderId,
                selectedEngineerId: selection.selectedEngineerId,
            }),
        [
            queries.snapshot,
            queries.plan,
            queries.compareMetrics,
            selection.selectedOrderId,
            selection.selectedEngineerId,
        ]
    );

    const buildPlan = useBuildPlan({
        scenarioId,
        snapshot: queries.currentSnapshot,
        planId: queries.planId,
        onReload: queries.reload,
    });
    const planEvent = useApplyPlanEvent({
        scenarioId,
        planId: queries.planId,
        snapshot: queries.currentSnapshot,
        plan: queries.plan,
        selectedOrderId: selection.selectedOrderId,
        onReload: queries.reload,
    });
    const crew = useUpdateCrew({
        scenarioId,
        snapshot: queries.currentSnapshot,
        onReload: queries.reload,
    });

    const runStatus: TypeOrNull<Run['status']> =
        buildPlan.runStatus ?? planEvent.runStatus;
    const eventPending = planEvent.pending;
    const showRunBanner =
        eventPending && Boolean(runStatus) && runStatus !== 'succeeded';

    const selectOrder = (id: TypeOrNull<string>) => {
        const engineerId = id ? view.engineerByOrder.get(id) : undefined;
        if (id) {
            selection.focusEngineer(engineerId ?? null);
        }
        selection.selectOrder(id);
        if (!engineerId) {
            selection.setFilter(selection.filter);
        }
    };
    const showUnassigned = () => {
        selection.focusEngineer(null);
        selection.selectOrder(null);
        selection.setFilter('unassigned');
    };

    const handleOrderEvent = (input: PlanEventInput) => {
        planEvent.apply(input);
    };

    const handlePatchEngineer = (engineerId: string, patch: CrewPatchInput) => {
        crew.update(engineerId, patch);
    };

    const handleEngineerUnavailable = (
        engineerId: string,
        occurredAt: string
    ) => {
        planEvent.apply({
            kind: 'engineer_unavailable',
            engineerId,
            occurredAt,
        });
    };

    return {
        snapshot: queries.snapshot,
        plan: queries.plan,
        compareMetrics: queries.compareMetrics,
        compareSource: queries.compareSource,
        isLoading: queries.isLoading,
        mapModel,
        mapFitToken: [
            scenarioId,
            selection.selectedEngineerId ?? 'overview',
            selection.panelTab === 'orders' ? selection.filter : 'all',
        ].join(':'),
        ...view,
        ...selection,
        selectOrder,
        setFilter: (filter: typeof selection.filter) => {
            selection.focusEngineer(null);
            selection.setFilter(filter);
        },
        onMarkerClick: (id: string) => {
            if (id !== 'office') {
                selectOrder(id);
            }
        },
        showUnassigned,
        runStatus,
        runStatusLabel: runStatus ? runStatusLabel[runStatus] : undefined,
        eventPending,
        showRunBanner,
        buildPending: buildPlan.pending,
        buildErrorMessage: buildPlan.errorMessage,
        crewPending: crew.pending || eventPending || buildPlan.pending,
        handleClearCrew: selection.clearCrew,
        handleBuildPlan: buildPlan.build,
        handleOrderEvent: queries.plan ? handleOrderEvent : undefined,
        handlePatchEngineer,
        handleEngineerUnavailable,
    };
};
