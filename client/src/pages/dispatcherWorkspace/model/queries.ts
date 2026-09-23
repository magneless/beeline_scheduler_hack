import { useQuery, useQueryClient } from '@tanstack/react-query';

import { getPlan, getScenario } from 'shared/api';

export const useWorkspaceQueries = (scenarioId: string) => {
    const queryClient = useQueryClient();

    const scenarioQuery = useQuery({
        queryKey: ['scenarios', scenarioId, 'current'],
        queryFn: () => getScenario(scenarioId),
        enabled: Boolean(scenarioId),
    });
    const planId = scenarioQuery.data?.current_plan_id ?? null;
    const planQuery = useQuery({
        queryKey: ['plans', scenarioId, planId],
        queryFn: () => getPlan(planId!),
        enabled: Boolean(scenarioId && planId),
    });
    const previousPlanId = planQuery.data?.base_plan_id ?? null;
    const previousPlanQuery = useQuery({
        queryKey: ['plans', scenarioId, previousPlanId],
        queryFn: () => getPlan(previousPlanId!),
        enabled: Boolean(scenarioId && previousPlanId),
    });
    const planRevision = planQuery.data?.snapshot_revision;
    const currentRevision = scenarioQuery.data?.snapshot.revision;
    const displayQuery = useQuery({
        queryKey: ['scenarios', scenarioId, planRevision],
        queryFn: () => getScenario(scenarioId, planRevision),
        enabled:
            Boolean(scenarioId) &&
            planRevision !== undefined &&
            planRevision !== currentRevision,
    });

    const currentSnapshot = scenarioQuery.data?.snapshot;
    const snapshot = displayQuery.data?.snapshot ?? currentSnapshot;
    const plan = planId ? planQuery.data : undefined;
    const compareSource = plan?.baseline_metrics
        ? ('baseline' as const)
        : previousPlanQuery.data?.metrics
          ? ('previous' as const)
          : null;
    const compareMetrics =
        plan?.baseline_metrics ?? previousPlanQuery.data?.metrics ?? null;

    const reload = async () => {
        await queryClient.invalidateQueries({
            queryKey: ['scenarios', scenarioId],
        });
        await queryClient.invalidateQueries({
            queryKey: ['plans', scenarioId],
        });
    };

    return {
        snapshot,
        currentSnapshot,
        plan,
        planId,
        compareMetrics,
        compareSource,
        isLoading: scenarioQuery.isLoading,
        reload,
    };
};
