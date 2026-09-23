import {
    type Engineer,
    type Equipment,
    type Order,
} from 'shared/api/types/contracts';
import { equipmentLabel, skillLabel, transportLabel } from 'shared/lib/config';

export const crewCompatibility = (
    order: Order,
    engineer: Engineer,
    stock: Partial<Record<Equipment, number>>,
    asOf?: string
): string[] => {
    const reasons: string[] = [];
    if (!engineer.available) {
        reasons.push('Бригада недоступна');
    }
    const missing = order.required_skills.filter(
        (skill) => !engineer.skills.includes(skill)
    );
    if (missing.length) {
        reasons.push(
            `Нет навыка: ${missing.map((skill) => skillLabel[skill] ?? skill).join(', ')}`
        );
    }
    if (
        order.required_transport &&
        order.required_transport !== engineer.transport
    ) {
        reasons.push(
            `Нужен транспорт: ${transportLabel[order.required_transport]}`
        );
    }
    for (const [code, count] of Object.entries(
        order.equipment_required
    ) as Array<[Equipment, number]>) {
        const available = stock[code] ?? 0;
        if (available < count) {
            reasons.push(
                `${equipmentLabel[code]}: нужно ${count}, осталось ${available}`
            );
        }
    }
    // The customer window limits the START; only the shift limits completion.
    const earliest = Math.max(
        Date.parse(order.window.start),
        Date.parse(order.received_at),
        Date.parse(engineer.shift.start),
        asOf ? Date.parse(asOf) : -Infinity
    );
    if (
        earliest > Date.parse(order.window.end) ||
        earliest + order.service_sec * 1000 > Date.parse(engineer.shift.end)
    ) {
        reasons.push(
            'Работа не помещается в смену и окно заявки даже без дороги'
        );
    }
    return reasons;
};
