import {
    apiGet,
    apiPost,
    apiPostWithProgress,
    type CalculationCallbacks,
} from 'shared/api/instance/httpClient';
import {
    type Plan,
    type PlanEvent,
    type PlanProposal,
    type Run,
    type Snapshot,
    type SolveMode,
} from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';

import {
    getAcceptProposalUrl,
    getBuildPlanUrl,
    getComparePlansUrl,
    getCurrentProposalUrl,
    getPlanEventsUrl,
    getPlanUrl,
    getProposalsUrl,
    getRunUrl,
} from '../../getUrl';

export const buildPlan = (input: {
    scenarioId: string;
    requestId: string;
    solveMode?: SolveMode;
    snapshotRevision: number;
    expectedCurrentPlanId: TypeOrNull<string>;
}) =>
    apiPost<{ run_id: string }>(getBuildPlanUrl(input.scenarioId), {
        request_id: input.requestId,
        solve_mode: input.solveMode,
        snapshot_revision: input.snapshotRevision,
        expected_current_plan_id: input.expectedCurrentPlanId,
    });

export type PlanComparison = {
    snapshot: Snapshot;
    baseline: Plan;
    optimized: Plan;
};

export const comparePlans = (
    scenarioId: string,
    currentPlanId: string,
    callbacks?: CalculationCallbacks
) =>
    apiPostWithProgress<PlanComparison>(
        getComparePlansUrl(scenarioId),
        {
            expected_current_plan_id: currentPlanId,
        },
        callbacks
    );

export const getRun = (runId: string) => apiGet<Run>(getRunUrl(runId));

export const getPlan = (planId: string) => apiGet<Plan>(getPlanUrl(planId));

export const createPlanProposal = (
    input: {
        scenarioId: string;
        requestId: string;
        snapshotRevision: number;
        expectedCurrentPlanId: TypeOrNull<string>;
        solveMode?: SolveMode;
        event?: PlanEvent;
        pendingRevision?: number;
    },
    callbacks?: CalculationCallbacks
) =>
    apiPostWithProgress<PlanProposal>(
        getProposalsUrl(input.scenarioId),
        {
            request_id: input.requestId,
            snapshot_revision: input.snapshotRevision,
            expected_current_plan_id: input.expectedCurrentPlanId,
            solve_mode: input.solveMode,
            ...(input.event ? { event: input.event } : {}),
            ...(input.pendingRevision !== undefined
                ? { pending_revision: input.pendingRevision }
                : {}),
        },
        callbacks
    );

export const getCurrentPlanProposal = (scenarioId: string) =>
    apiGet<PlanProposal | null>(getCurrentProposalUrl(scenarioId));

export const acceptPlanProposal = (input: {
    proposalId: string;
    requestId: string;
    optionKey: string;
}) =>
    apiPost<Plan>(getAcceptProposalUrl(input.proposalId), {
        request_id: input.requestId,
        option_key: input.optionKey,
    });

export const postPlanEvent = (input: {
    planId: string;
    requestId: string;
    solveMode?: SolveMode;
    snapshotRevision: number;
    event: PlanEvent;
}) =>
    apiPost<{ run_id: string }>(getPlanEventsUrl(input.planId), {
        request_id: input.requestId,
        solve_mode: input.solveMode,
        snapshot_revision: input.snapshotRevision,
        event: input.event,
    });

const wait = (ms: number) =>
    new Promise((resolve) => {
        window.setTimeout(resolve, ms);
    });

export const waitForRun = async (
    runId: string,
    onStatus?: (run: Run) => void
) => {
    const deadline = Date.now() + 6 * 60 * 1000;
    let interval = 250;

    while (Date.now() < deadline) {
        const run = await getRun(runId);

        onStatus?.(run);

        if (run.status === 'succeeded' || run.status === 'failed') {
            return run;
        }

        await wait(interval);
        interval = Math.min(1000, interval * 2);
    }

    throw new Error(
        'Расчёт ещё выполняется. Обновите сценарий, чтобы получить результат.'
    );
};
