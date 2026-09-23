export const getDemoDatasetsUrl = () => '/demo-datasets';

export const getScenariosUrl = () => '/scenarios';

export const getScenarioImportUrl = () => '/scenarios/import';

export const getScenarioUrl = (scenarioId: string, revision?: number) => {
    const query = revision === undefined ? '' : `?revision=${revision}`;
    return `/scenarios/${scenarioId}${query}`;
};

export const getEngineerUrl = (scenarioId: string, engineerId: string) =>
    `/scenarios/${scenarioId}/engineers/${engineerId}`;
export const getEngineerImportUrl = (scenarioId: string) =>
    `/scenarios/${scenarioId}/engineers/import`;

export const getBuildPlanUrl = (scenarioId: string) =>
    `/scenarios/${scenarioId}/plans`;

export const getRunUrl = (runId: string) => `/runs/${runId}`;

export const getPlanUrl = (planId: string) => `/plans/${planId}`;

export const getPlanEventsUrl = (planId: string) => `/plans/${planId}/events`;
