import {
    type Engineer,
    type Equipment,
    type Order,
    type Visit,
} from 'shared/api/types/contracts';
import { equipmentLabel, skillLabel, transportLabel } from 'shared/lib/config';
import { formatClock } from 'shared/lib/utils';

export type AssignmentFactor = {
    id: 'skill' | 'transport' | 'equipment' | 'window' | 'shift';
    ok: boolean;
    label: string;
    detail: string;
};

export const assignmentFactors = (
    order: Order,
    engineer: Engineer,
    visit: Visit | undefined,
    timezone: string,
    remaining?: Partial<Record<Equipment, number>>
): AssignmentFactor[] => {
    const stock =
        remaining !== undefined ? remaining : engineer.equipment_stock;
    const missingSkills = order.required_skills.filter(
        (skill) => !engineer.skills.includes(skill)
    );
    const transportOk =
        !order.required_transport ||
        order.required_transport === engineer.transport;
    const missingEquipment = (
        Object.entries(order.equipment_required) as Array<[Equipment, number]>
    ).filter(([code, count]) => (stock[code] ?? 0) < count);
    const windowOk = visit
        ? visit.start_at >= order.window.start &&
          visit.start_at <= order.window.end
        : false;
    const shiftOk = visit
        ? visit.start_at >= engineer.shift.start &&
          visit.end_at <= engineer.shift.end
        : false;

    return [
        {
            id: 'skill',
            ok: missingSkills.length === 0,
            label: 'Навык',
            detail: missingSkills.length
                ? `нет ${missingSkills.map((skill) => skillLabel[skill] ?? skill).join(', ')}`
                : order.required_skills
                      .map((skill) => skillLabel[skill] ?? skill)
                      .join(', ') || 'не требуется',
        },
        {
            id: 'transport',
            ok: transportOk,
            label: 'Транспорт',
            detail: order.required_transport
                ? transportLabel[order.required_transport]
                : 'любой',
        },
        {
            id: 'equipment',
            ok: missingEquipment.length === 0,
            label: 'Склад',
            detail: missingEquipment.length
                ? `мало ${missingEquipment
                      .map(([code]) => equipmentLabel[code])
                      .join(', ')}`
                : Object.keys(order.equipment_required).length
                  ? 'хватает на заявку'
                  : 'не требуется',
        },
        {
            id: 'window',
            ok: windowOk,
            label: 'Окно',
            detail: visit
                ? `старт ${formatClock(visit.start_at, timezone)} в ${formatClock(
                      order.window.start,
                      timezone
                  )}–${formatClock(order.window.end, timezone)}`
                : 'визит ещё не в плане',
        },
        {
            id: 'shift',
            ok: shiftOk,
            label: 'Смена',
            detail: visit
                ? `до ${formatClock(visit.end_at, timezone)}, смена до ${formatClock(engineer.shift.end, timezone)}`
                : 'визит ещё не в плане',
        },
    ];
};
