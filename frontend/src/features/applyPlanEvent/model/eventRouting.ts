import type { Plan } from 'shared/api/types/contracts';

import type { PlanEventInput } from './types';

export const isUnassignedCancellation = (
    input: PlanEventInput,
    plan: Pick<Plan, 'unassigned'> | undefined,
    orderId: string | null
) => {
    const ids =
        input.kind === 'cancel_many'
            ? input.orderIds
            : input.kind === 'cancel' && orderId
              ? [orderId]
              : [];
    return (
        ids.length > 0 &&
        ids.every((id) => plan?.unassigned.some((item) => item.order_id === id))
    );
};
