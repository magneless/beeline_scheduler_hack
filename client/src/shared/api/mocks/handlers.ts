import { http, HttpResponse } from 'msw';

import {
    type Engineer,
    type Plan,
    type PlanEvent,
    type ScenarioView,
} from 'shared/api/types/contracts';
import { env } from 'shared/config/env';

import {
    applyEngineerPatch,
    applyPlanEvent,
    createDemoPlan,
    createScenarioView,
    demoDatasets,
    isScenarioBlocked,
    scanImportIssues,
} from './demoDay';

const memory = {
    scenarios: new Map<string, ScenarioView>(),
    revisions: new Map<string, ScenarioView['snapshot']>(),
    plans: new Map<string, Plan>(),
    runs: new Map<string, { scenario_id: string; plan_id: string }>(),
    runPolls: new Map<string, number>(),
    planCount: 0,
};

const api = `*${env.apiUrl}`;

const revisionKey = (scenarioId: string, revision: number) =>
    `${scenarioId}:${revision}`;

const remember = (scenario: ScenarioView) => {
    memory.scenarios.set(scenario.snapshot.scenario_id, scenario);
    memory.revisions.set(
        revisionKey(scenario.snapshot.scenario_id, scenario.snapshot.revision),
        structuredClone(scenario.snapshot)
    );
};

const resolveRegion = (value: string) =>
    value.replace(/^scenario-/, '').replace(/-import$/, '');

const errorBody = (status: number, code: string, message: string) =>
    HttpResponse.json({ code, message, details: {} }, { status });

export const handlers = [
    http.get(`${api}/demo-datasets`, () => {
        return HttpResponse.json(demoDatasets);
    }),
    http.post(`${api}/scenarios`, async ({ request }) => {
        const body = (await request.json()) as { demo_dataset_id: string };
        const scenario = createScenarioView(body.demo_dataset_id);

        remember(scenario);

        return HttpResponse.json(scenario, { status: 201 });
    }),
    http.post(`${api}/scenarios/import`, async ({ request }) => {
        const form = await request.formData();
        const file = form.get('file');
        const regionId = String(form.get('region_id') ?? '');
        const date = String(form.get('date') ?? '');

        if (!(file instanceof File) || file.size === 0 || !regionId || !date) {
            return errorBody(
                400,
                'INVALID_INPUT',
                'Нечитаемый CSV или некорректный multipart'
            );
        }

        const csvText = await file.text();
        const issues = scanImportIssues(csvText);
        const scenario = createScenarioView(regionId, {
            scenarioId: `scenario-${resolveRegion(regionId)}-import`,
            date,
            issues,
        });

        remember(scenario);

        return HttpResponse.json(scenario, { status: 201 });
    }),
    http.get(`${api}/scenarios/:id`, ({ params, request }) => {
        const scenarioId = String(params.id);
        const revision = new URL(request.url).searchParams.get('revision');
        let scenario = memory.scenarios.get(scenarioId);

        if (!scenario) {
            scenario = createScenarioView(resolveRegion(scenarioId), {
                scenarioId,
            });
            remember(scenario);
        }

        if (revision) {
            const snapshot = memory.revisions.get(
                revisionKey(scenarioId, Number(revision))
            );

            if (!snapshot) {
                return errorBody(404, 'NOT_FOUND', 'Ревизия не найдена');
            }

            return HttpResponse.json({
                snapshot,
                current_plan_id: scenario.current_plan_id,
            });
        }

        return HttpResponse.json(scenario);
    }),
    http.patch(
        `${api}/scenarios/:id/engineers/:engineerId`,
        async ({ params, request }) => {
            const scenarioId = String(params.id);
            const engineerId = String(params.engineerId);
            const scenario = memory.scenarios.get(scenarioId);
            const plan = scenario?.current_plan_id
                ? memory.plans.get(scenario.current_plan_id)
                : null;
            const body = (await request.json()) as {
                expected_revision?: number;
                skills?: Engineer['skills'];
                transport?: Engineer['transport'];
                shift?: Engineer['shift'];
                available?: boolean;
                equipment_stock?: Engineer['equipment_stock'];
            };

            if (!scenario) {
                return errorBody(404, 'NOT_FOUND', 'Сценарий не найден');
            }

            if (isScenarioBlocked(scenario, plan)) {
                return errorBody(
                    409,
                    'EVENT_CONFLICT',
                    'День уже начался, правьте бригаду через событие'
                );
            }

            if (body.expected_revision !== scenario.snapshot.revision) {
                return errorBody(
                    409,
                    'STALE_VERSION',
                    'Исходные данные или текущий план изменились'
                );
            }

            const next = applyEngineerPatch(scenario, engineerId, body);

            if (!next) {
                return errorBody(404, 'NOT_FOUND', 'Инженер не найден');
            }

            remember(next);

            return HttpResponse.json(next);
        }
    ),
    http.post(`${api}/scenarios/:id/plans`, async ({ params, request }) => {
        const scenarioId = String(params.id);
        const scenario = memory.scenarios.get(scenarioId);

        if (!scenario) {
            return errorBody(404, 'NOT_FOUND', 'Сценарий не найден');
        }

        const body = (await request.json()) as {
            request_id?: string;
            snapshot_revision?: number;
            expected_current_plan_id?: string | null;
        };

        if (
            body.snapshot_revision !== undefined &&
            body.snapshot_revision !== scenario.snapshot.revision
        ) {
            return errorBody(
                409,
                'STALE_VERSION',
                'Исходные данные или текущий план изменились'
            );
        }

        if (
            body.expected_current_plan_id !== undefined &&
            body.expected_current_plan_id !== scenario.current_plan_id
        ) {
            return errorBody(
                409,
                'STALE_VERSION',
                'Исходные данные или текущий план изменились'
            );
        }

        const currentPlan = scenario.current_plan_id
            ? (memory.plans.get(scenario.current_plan_id) ?? null)
            : null;

        if (isScenarioBlocked(scenario, currentPlan)) {
            return errorBody(
                409,
                'EVENT_CONFLICT',
                'Повторный расчёт после события недоступен'
            );
        }

        memory.planCount += 1;
        const plan = createDemoPlan(scenario);
        plan.id = `plan-${memory.planCount}`;
        plan.snapshot_revision = scenario.snapshot.revision;
        memory.plans.set(plan.id, plan);
        scenario.current_plan_id = plan.id;
        const runId = `run-${memory.planCount}`;

        memory.runs.set(runId, {
            scenario_id: scenarioId,
            plan_id: plan.id,
        });

        return HttpResponse.json({ run_id: runId }, { status: 202 });
    }),
    http.post(`${api}/plans/:id/events`, async ({ params, request }) => {
        const plan = memory.plans.get(String(params.id));
        const body = (await request.json()) as {
            snapshot_revision: number;
            event: PlanEvent;
        };

        if (!plan) {
            return errorBody(404, 'NOT_FOUND', 'План не найден');
        }

        const scenario = [...memory.scenarios.values()].find(
            (item) => item.current_plan_id === plan.id
        );

        if (!scenario) {
            return errorBody(404, 'NOT_FOUND', 'Сценарий не найден');
        }

        if (body.snapshot_revision !== scenario.snapshot.revision) {
            return errorBody(
                409,
                'STALE_VERSION',
                'Исходные данные или текущий план изменились'
            );
        }

        memory.planCount += 1;
        const nextPlanId = `plan-${memory.planCount}`;
        const runId = `run-${memory.planCount}`;
        const applied = applyPlanEvent(scenario, plan, body.event, nextPlanId);

        remember(applied.scenario);
        memory.plans.set(applied.plan.id, applied.plan);
        memory.runs.set(runId, {
            scenario_id: applied.scenario.snapshot.scenario_id,
            plan_id: applied.plan.id,
        });

        return HttpResponse.json({ run_id: runId }, { status: 202 });
    }),
    http.get(`${api}/runs/:id`, ({ params }) => {
        const runId = String(params.id);
        const run = memory.runs.get(runId);
        const polls = memory.runPolls.get(runId) ?? 0;

        memory.runPolls.set(runId, polls + 1);

        const scenario = [...memory.scenarios.values()].find(
            (item) => item.current_plan_id
        );

        return HttpResponse.json({
            id: runId,
            scenario_id:
                run?.scenario_id ??
                scenario?.snapshot.scenario_id ??
                'scenario-east',
            status: polls === 0 ? 'running' : 'succeeded',
            plan_id: run?.plan_id ?? scenario?.current_plan_id ?? 'plan-1',
            error: null,
        });
    }),
    http.get(`${api}/plans/:id`, ({ params }) => {
        const plan = memory.plans.get(String(params.id));

        if (!plan) {
            return errorBody(404, 'NOT_FOUND', 'План не найден');
        }

        return HttpResponse.json(plan);
    }),
];
