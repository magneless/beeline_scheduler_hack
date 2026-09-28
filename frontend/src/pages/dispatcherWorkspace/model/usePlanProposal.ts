import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { createEvent, type PlanEventInput } from 'features/applyPlanEvent';
import {
    acceptPlanProposal,
    createPlanProposal,
    getCurrentPlanProposal,
} from 'shared/api';
import {
    type Plan,
    type Snapshot,
    type SolveMode,
} from 'shared/api/types/contracts';
import {
    commandErrorMessage,
    isStaleVersionError,
} from 'shared/lib/commandErrors';
import { type CalculationState } from 'shared/ui/calculationProgress';

type Params = {
    scenarioId: string;
    snapshot?: Snapshot;
    plan?: Plan;
    planId: string | null;
    solveMode: SolveMode;
    selectedOrderId: string | null;
    onReload: () => Promise<void>;
    onProposalReady: () => void;
};

export const usePlanProposal = ({
    scenarioId,
    snapshot,
    plan,
    planId,
    solveMode,
    selectedOrderId,
    onReload,
    onProposalReady,
}: Params) => {
    const queryClient = useQueryClient();
    const [calculation, setCalculation] = useState<
        (CalculationState & { scenarioId: string }) | null
    >(null);
    const request = useRef<AbortController | null>(null);
    const activeRequestToken = useRef<string | null>(null);
    const currentScenarioId = useRef(scenarioId);
    currentScenarioId.current = scenarioId;
    useEffect(() => {
        return () => {
            request.current?.abort();
            request.current = null;
        };
    }, [scenarioId]);
    const queryKey = ['scenarios', scenarioId, 'proposal'];
    const query = useQuery({
        queryKey,
        queryFn: () => getCurrentPlanProposal(scenarioId),
        enabled: Boolean(scenarioId),
        retry: false,
    });
    const create = useMutation({
        mutationFn: async ({
            input,
            token,
            startedFor,
        }: {
            input?: PlanEventInput;
            token: string;
            startedFor: string;
        }) => {
            if (
                startedFor !== currentScenarioId.current ||
                token !== activeRequestToken.current
            ) {
                throw new DOMException('Запрос отменён', 'AbortError');
            }
            if (!snapshot) {
                throw new Error('Сценарий ещё не загружен');
            }
            if (snapshot.engineers.length === 0) {
                throw new Error('Добавьте бригады');
            }
            if (input && !plan) {
                throw new Error('Сначала примите план дня');
            }
            const event =
                input && plan
                    ? createEvent(input, snapshot, plan, selectedOrderId)
                    : undefined;
            const controller = new AbortController();
            request.current = controller;
            const startedAt = Date.now();
            setCalculation({
                startedAt,
                lastEventAt: startedAt,
                scenarioId: startedFor,
            });
            try {
                return await createPlanProposal(
                    {
                        scenarioId,
                        requestId: crypto.randomUUID(),
                        snapshotRevision: snapshot.revision,
                        expectedCurrentPlanId: planId,
                        solveMode,
                        event,
                    },
                    {
                        signal: controller.signal,
                        onHeartbeat: () => {
                            if (
                                !controller.signal.aborted &&
                                token === activeRequestToken.current &&
                                startedFor === currentScenarioId.current
                            ) {
                                setCalculation(
                                    (current) =>
                                        current && {
                                            ...current,
                                            lastEventAt: Date.now(),
                                        }
                                );
                            }
                        },
                        onProgress: (progress) => {
                            if (
                                !controller.signal.aborted &&
                                token === activeRequestToken.current &&
                                startedFor === currentScenarioId.current
                            ) {
                                setCalculation(
                                    (current) =>
                                        current && {
                                            ...current,
                                            progress,
                                            lastEventAt: Date.now(),
                                        }
                                );
                            }
                        },
                    }
                );
            } finally {
                if (request.current === controller) {
                    request.current = null;
                }
            }
        },
        onSuccess: (proposal, { token, startedFor }) => {
            if (
                token !== activeRequestToken.current ||
                startedFor !== currentScenarioId.current
            ) {
                return;
            }
            queryClient.setQueryData(queryKey, proposal);
            onProposalReady();
        },
        onError: (error, { token, startedFor }) => {
            if (
                token !== activeRequestToken.current ||
                startedFor !== currentScenarioId.current
            ) {
                return;
            }
            if (error instanceof Error && error.name === 'AbortError') {
                return;
            }
            toast.error(commandErrorMessage(error));
            if (isStaleVersionError(error)) {
                void onReload();
            }
        },
        onSettled: (_result, _error, { token, startedFor }) => {
            if (
                token === activeRequestToken.current &&
                startedFor === currentScenarioId.current
            ) {
                activeRequestToken.current = null;
                setCalculation(null);
            }
        },
    });
    const accept = useMutation({
        mutationFn: async (optionKey: string) => {
            if (!query.data) {
                throw new Error('Нет варианта для принятия');
            }
            return acceptPlanProposal({
                proposalId: query.data.id,
                requestId: `accept-${query.data.id}-${optionKey}`,
                optionKey,
            });
        },
        onSuccess: async () => {
            queryClient.setQueryData(queryKey, null);
            await onReload();
            toast.success('Рабочий план принят');
        },
        onError: (error) => {
            toast.error(commandErrorMessage(error));
            if (isStaleVersionError(error)) {
                void onReload();
            }
        },
    });

    return {
        proposal: query.data ?? null,
        create: (input?: PlanEventInput) => {
            request.current?.abort();
            const token = crypto.randomUUID();
            activeRequestToken.current = token;
            create.mutate({ input, token, startedFor: scenarioId });
        },
        accept: (optionKey: string) => accept.mutate(optionKey),
        creating: create.isPending,
        calculation:
            calculation?.scenarioId === scenarioId ? calculation : null,
        accepting: accept.isPending,
        createError: create.error
            ? commandErrorMessage(create.error)
            : undefined,
    };
};
