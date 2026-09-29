import {
    apiGet,
    apiPatch,
    apiPost,
    apiPostWithProgress,
    type CalculationCallbacks,
} from 'shared/api/instance/httpClient';
import {
    type Engineer,
    type Point,
    type ScenarioSummary,
    type ScenarioView,
} from 'shared/api/types/contracts';

import {
    getEngineerImportUrl,
    getEngineerUrl,
    getScenarioImportUrl,
    getScenariosUrl,
    getScenarioUrl,
} from '../../getUrl';

export const getScenarios = () =>
    apiGet<{ items: ScenarioSummary[] }>(getScenariosUrl());

export const createScenario = (
    demoDatasetId: string,
    callbacks?: CalculationCallbacks
) =>
    apiPostWithProgress<ScenarioView>(
        getScenariosUrl(),
        {
            demo_dataset_id: demoDatasetId,
        },
        callbacks
    );

export const importScenario = (
    input: {
        file: File;
        regionId?: string;
        date: string;
        engineersFile?: File;
        officeAddress?: string;
        officePoint?: Point;
    },
    callbacks?: CalculationCallbacks
) => {
    const body = new FormData();

    body.append('file', input.file);
    if (input.regionId) {
        body.append('region_id', input.regionId);
    }
    body.append('date', input.date);
    if (input.engineersFile) {
        body.append('engineers_file', input.engineersFile);
    }
    if (input.officeAddress) {
        body.append('office_address', input.officeAddress);
    }
    if (input.officePoint) {
        body.append('office_point', JSON.stringify(input.officePoint));
    }

    return apiPostWithProgress<ScenarioView>(
        getScenarioImportUrl(),
        body,
        callbacks
    );
};

export const getScenario = (scenarioId: string, revision?: number) =>
    apiGet<ScenarioView>(getScenarioUrl(scenarioId, revision));

export type EngineerPatch = {
    scenarioId: string;
    engineerId: string;
    expectedRevision: number;
    skills?: Engineer['skills'];
    transport?: Engineer['transport'];
    shift?: Engineer['shift'];
    available?: boolean;
    reserve?: boolean;
    equipment_stock?: Engineer['equipment_stock'];
};

export const patchEngineer = ({
    scenarioId,
    engineerId,
    expectedRevision,
    ...fields
}: EngineerPatch) => {
    const body: Record<string, unknown> = {
        expected_revision: expectedRevision,
    };

    if (fields.skills !== undefined) {
        body.skills = fields.skills;
    }

    if (fields.transport !== undefined) {
        body.transport = fields.transport;
    }

    if (fields.shift !== undefined) {
        body.shift = fields.shift;
    }

    if (fields.available !== undefined) {
        body.available = fields.available;
    }

    if (fields.reserve !== undefined) {
        body.reserve = fields.reserve;
    }

    if (fields.equipment_stock !== undefined) {
        body.equipment_stock = fields.equipment_stock;
    }

    return apiPatch<ScenarioView>(getEngineerUrl(scenarioId, engineerId), body);
};

export const importEngineers = (input: {
    scenarioId: string;
    file: File;
    expectedRevision: number;
}) => {
    const body = new FormData();
    body.append('file', input.file);
    body.append('expected_revision', String(input.expectedRevision));
    return apiPost<ScenarioView>(getEngineerImportUrl(input.scenarioId), body);
};
