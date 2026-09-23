import { apiGet, apiPost } from 'shared/api/instance/httpClient';
import {
    type Plan,
    type PlanEvent,
    type Run,
} from 'shared/api/types/contracts';
import { type TypeOrNull } from 'shared/lib/types';

import {
    getBuildPlanUrl,
    getPlanEventsUrl,
    getPlanUrl,
    getRunUrl,
} from '../../getUrl';

export const buildPlan = (input: {
    scenarioId: string;
    requestId: string;
    snapshotRevision: number;
    expectedCurrentPlanId: TypeOrNull<string>;
}) =>
    apiPost<{ run_id: string }>(getBuildPlanUrl(input.scenarioId), {
        request_id: input.requestId,
        snapshot_revision: input.snapshotRevision,
        expected_current_plan_id: input.expectedCurrentPlanId,
    });

export const getRun = (runId: string) => apiGet<Run>(getRunUrl(runId));

export const getPlan = (planId: string) => apiGet<Plan>(getPlanUrl(planId));

export const postPlanEvent = (input: {
    planId: string;
    requestId: string;
    snapshotRevision: number;
    event: PlanEvent;
}) =>
    apiPost<{ run_id: string }>(getPlanEventsUrl(input.planId), {
        request_id: input.requestId,
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

    while (Date.now() < deadline) {
        const run = await getRun(runId);

        onStatus?.(run);

        if (run.status === 'succeeded' || run.status === 'failed') {
            return run;
        }

        await wait(1000);
    }

    throw new Error(
        'Расчёт ещё выполняется. Обновите сценарий, чтобы получить результат.'
    );
};
