import { type Engineer } from 'shared/api/types/contracts';

export type CrewPatchInput = {
    skills: string[];
    transport: Engineer['transport'];
    shift: Engineer['shift'];
    available: boolean;
    equipment_stock: Engineer['equipment_stock'];
};
