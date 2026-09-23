import { waitForRun } from 'shared/api';
import { type Run } from 'shared/api/types/contracts';

export const runPlanCommand = async (
    getRunId: () => Promise<{ run_id: string }>,
    onStatus: (status: Run['status']) => void
) => {
    const accepted = await getRunId();
    const run = await waitForRun(accepted.run_id, (next) =>
        onStatus(next.status)
    );

    if (run.status !== 'succeeded' || !run.plan_id) {
        throw new Error(run.error?.message ?? 'Расчёт не завершился');
    }

    return run;
};
