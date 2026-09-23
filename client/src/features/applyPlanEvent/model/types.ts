import {
    type CancelReason,
    type OrderStatus,
} from 'shared/api/types/contracts';

export type PlanEventInput =
    | { kind: 'urgent'; occurredAt: string }
    | { kind: 'cancel'; occurredAt: string; reason: CancelReason }
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
