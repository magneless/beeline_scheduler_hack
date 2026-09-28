import {
    type CancelReason,
    type OrderStatus,
} from 'shared/api/types/contracts';

export type PlanEventInput =
    | {
          kind: 'new_order';
          occurredAt: string;
          orderType: 'urgent' | 'ordinary';
          locationId?: string;
          address?: string;
          restoreOrderId?: string;
          point?: import('shared/api/types/contracts').Point;
          workType: import('shared/api/types/contracts').WorkType;
          requiredSkills: string[];
          transport: import('shared/api/types/contracts').Transport | null;
          windowStart: string;
          windowEnd: string;
          serviceSec: number;
          equipment: Partial<
              Record<import('shared/api/types/contracts').Equipment, number>
          >;
      }
    | { kind: 'cancel'; occurredAt: string; reason: CancelReason }
    | {
          kind: 'cancel_many';
          orderIds: string[];
          occurredAt: string;
          reason: CancelReason;
      }
    | {
          kind: 'status';
          occurredAt: string;
          status: Extract<
              OrderStatus,
              'sent' | 'en_route' | 'in_progress' | 'completed'
          >;
          expectedEndAt?: string;
      }
    | {
          kind: 'engineer_unavailable';
          occurredAt: string;
          engineerId: string;
      };
