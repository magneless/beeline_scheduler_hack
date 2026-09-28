import { useEffect, useMemo, useState } from 'react';

import {
    isUnassignedCancellation,
    type PlanEventInput,
    useApplyPlanEvent,
} from 'features/applyPlanEvent';
import { type CrewPatchInput, useUpdateCrew } from 'features/updateCrew';
import { type Run, type SolveMode } from 'shared/api/types/contracts';
import { runStatusLabel } from 'shared/lib/config';
import { type TypeOrNull } from 'shared/lib/types';

import { useWorkspaceQueries } from './queries';
import { useDispatcherWorkspaceStore } from './store';
import { usePlanProposal } from './usePlanProposal';
import { useWorkspaceSelection } from './useWorkspaceSelection';
import { buildMapModel } from '../lib/utils';
import { buildWorkspaceView } from '../lib/viewModel';

export const useDispatcherWorkspace = (scenarioId: string) => {
    const selection = useWorkspaceSelection();
    const queries = useWorkspaceQueries(scenarioId);
    const [proposalOpen, setProposalOpen] = useState(true);
    const [scenarioTime, setScenarioTime] = useState('');
    useEffect(() => {
        useDispatcherWorkspaceStore.getState().resetSelection();
        setProposalOpen(true);
        setScenarioTime('');
    }, [scenarioId]);

    const mapModel = useMemo(
        () =>
            queries.snapshot
                ? buildMapModel(
                      queries.snapshot,
                      queries.plan,
                      selection.selectedEngineerId,
                      selection.panelTab !== 'crews' &&
                          selection.filter === 'unassigned' &&
                          !selection.selectedEngineerId,
                      selection.selectedOrderId
                  )
                : { markers: [], polylines: [] },
        [
            queries.snapshot,
            queries.plan,
            selection.selectedEngineerId,
            selection.panelTab,
            selection.filter,
            selection.selectedOrderId,
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
    const planAsOf = queries.plan?.as_of;
    useEffect(() => {
        if (planAsOf) {
            setScenarioTime((current) =>
                current && Date.parse(current) < Date.parse(planAsOf)
                    ? planAsOf
                    : current
            );
        }
    }, [planAsOf]);
    const effectiveScenarioTime = scenarioTime || view.occurredAtDefault;
    const inTransitOrderIds = new Set<string>();
    const atMillis = Date.parse(effectiveScenarioTime);
    if (Number.isFinite(atMillis)) {
        const orderById = new Map(
            queries.snapshot?.orders.map((order) => [order.id, order]) ?? []
        );
        queries.plan?.routes.forEach((route) => {
            const next = route.visits.find((visit) => {
                const status = orderById.get(visit.order_id)?.status;
                return status !== 'completed' && status !== 'cancelled';
            });
            if (!next) {
                return;
            }
            const order = orderById.get(next.order_id);
            if (order?.status !== 'active') {
                return;
            }
            const leg = route.legs
                .filter(
                    (item) =>
                        item.to_location_id === order.location_id &&
                        Date.parse(item.end_at) <= Date.parse(next.arrival_at)
                )
                .sort(
                    (left, right) =>
                        Date.parse(right.end_at) - Date.parse(left.end_at)
                )[0];
            if (leg && Date.parse(leg.start_at) <= atMillis) {
                inTransitOrderIds.add(next.order_id);
            }
        });
    }

    const savedSolveMode =
        queries.plan?.solve_mode ??
        (queries.plan?.issues.some((issue) => issue.code === 'BASELINE_ONLY')
            ? 'baseline'
            : 'optimized');
    const [algorithmChoice, setAlgorithmChoice] = useState<{
        scenarioId: string;
        planId: string | null;
        mode: SolveMode;
    }>();
    const solveMode =
        algorithmChoice?.scenarioId === scenarioId &&
        algorithmChoice.planId === queries.planId
            ? algorithmChoice.mode
            : savedSolveMode;
    const setSolveMode = (mode: SolveMode) =>
        setAlgorithmChoice({ scenarioId, planId: queries.planId, mode });

    const proposal = usePlanProposal({
        solveMode,
        scenarioId,
        snapshot: queries.currentSnapshot,
        planId: queries.planId,
        plan: queries.plan,
        selectedOrderId: selection.selectedOrderId,
        onReload: queries.reload,
        onProposalReady: () => setProposalOpen(true),
    });
    const planEvent = useApplyPlanEvent({
        solveMode,
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

    const runStatus: TypeOrNull<Run['status']> = planEvent.runStatus;
    const eventPending = planEvent.pending;
    const showRunBanner =
        eventPending &&
        Boolean(runStatus) &&
        !planEvent.calculation &&
        runStatus !== 'succeeded';

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
        selection.setPanelTab('both');
        selection.focusEngineer(null);
        selection.selectOrder(null);
        selection.setFilter('unassigned');
    };

    const handleOrderEvent = (input: PlanEventInput) => {
        if (
            input.kind === 'status' ||
            input.kind === 'cancel_many' ||
            isUnassignedCancellation(
                input,
                queries.plan,
                selection.selectedOrderId
            )
        ) {
            planEvent.apply(input);
        } else {
            proposal.create(input);
        }
    };

    const handlePatchEngineer = (engineerId: string, patch: CrewPatchInput) => {
        crew.update(engineerId, patch);
    };

    const handleEngineerUnavailable = (
        engineerId: string,
        occurredAt: string
    ) => {
        proposal.create({
            kind: 'engineer_unavailable',
            engineerId,
            occurredAt,
        });
    };

    return {
        proposal: proposal.proposal,
        proposalCalculation: proposal.calculation,
        runCalculation: planEvent.calculation,
        proposalOpen,
        setProposalOpen,
        acceptProposal: proposal.accept,
        proposalAccepting: proposal.accepting,
        scenarioTime: effectiveScenarioTime,
        inTransitOrderIds,
        setScenarioTime,
        solveMode,
        setSolveMode,
        savedSolveMode: queries.plan ? savedSolveMode : undefined,
        snapshot: queries.snapshot,
        plan: queries.plan,
        compareMetrics: queries.compareMetrics,
        compareSource: queries.compareSource,
        isLoading: queries.isLoading,
        mapModel,
        mapFitToken: [
            scenarioId,
            selection.selectedEngineerId ?? 'overview',
            selection.panelTab !== 'crews' ? selection.filter : 'all',
            selection.selectedOrderId &&
            view.unassigned.has(selection.selectedOrderId)
                ? selection.selectedOrderId
                : '',
        ].join(':'),
        ...view,
        ...selection,
        selectOrder,
        setFilter: (filter: typeof selection.filter) => {
            selection.focusEngineer(null);
            selection.selectOrder(null);
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
        eventPending: eventPending || proposal.creating || proposal.accepting,
        showRunBanner,
        buildPending: proposal.creating,
        buildErrorMessage: proposal.createError,
        crewPending:
            crew.pending ||
            eventPending ||
            proposal.creating ||
            proposal.accepting,
        handleClearCrew: () => {
            selection.focusEngineer(null);
            selection.selectOrder(null);
            selection.setFilter('all');
        },
        handleBuildPlan: () => proposal.create(),
        handleOrderEvent: queries.plan ? handleOrderEvent : undefined,
        handlePatchEngineer,
        handleEngineerUnavailable,
    };
};
