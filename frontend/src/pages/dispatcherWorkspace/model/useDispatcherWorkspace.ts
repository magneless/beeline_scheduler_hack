import { useMemo } from 'react';

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
import { useWorkspaceSelection } from './useWorkspaceSelection';
import { buildMapModel } from '../lib/utils';
import { buildWorkspaceView } from '../lib/viewModel';

export const useDispatcherWorkspace = (scenarioId: string) => {
    const selection = useWorkspaceSelection();
    const queries = useWorkspaceQueries(scenarioId);

    const mapModel = useMemo(
        () =>
            queries.snapshot
                ? buildMapModel(
                      queries.snapshot,
                      queries.plan,
                      selection.selectedEngineerId
                  )
                : { markers: [], polylines: [] },
        [queries.snapshot, queries.plan, selection.selectedEngineerId]
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

    const handleEmergency = (occurredAt: string) => {
        planEvent.apply({ kind: 'urgent', occurredAt });
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
        mapFitToken: queries.snapshot?.region_id ?? scenarioId,
        ...view,
        ...selection,
        runStatus,
        runStatusLabel: runStatus ? runStatusLabel[runStatus] : undefined,
        eventPending,
        showRunBanner,
        buildPending: buildPlan.pending,
        crewPending: crew.pending || eventPending,
        handleClearCrew: selection.clearCrew,
        handleEmergency,
        handleBuildPlan: buildPlan.build,
        handleOrderEvent: queries.plan ? handleOrderEvent : undefined,
        handlePatchEngineer,
        handleEngineerUnavailable,
    };
};
