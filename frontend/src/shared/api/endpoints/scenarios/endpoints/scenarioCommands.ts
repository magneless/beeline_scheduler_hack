import { apiGet, apiPatch, apiPost } from 'shared/api/instance/httpClient';
import { type Engineer, type ScenarioView } from 'shared/api/types/contracts';

import {
    getEngineerImportUrl,
    getEngineerUrl,
    getScenarioImportUrl,
    getScenariosUrl,
    getScenarioUrl,
} from '../../getUrl';

export const createScenario = (demoDatasetId: string) =>
    apiPost<ScenarioView>(getScenariosUrl(), {
        demo_dataset_id: demoDatasetId,
    });

export const importScenario = (input: {
    file: File;
    regionId: string;
    date: string;
}) => {
    const body = new FormData();

    body.append('file', input.file);
    body.append('region_id', input.regionId);
    body.append('date', input.date);

    return apiPost<ScenarioView>(getScenarioImportUrl(), body);
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
