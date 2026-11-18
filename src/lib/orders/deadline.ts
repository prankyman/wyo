import type { TOrderState } from '$lib/components/orders/types';

/** How many hours before an order's deadline the order list starts to warn: time enough to pack and hand the order over. */
export const DEADLINE_ATTENTION_HOURS = 6;

/** The states in which an order still waits to be handed to the marketplace, so its deadline still matters. */
const OPEN_STATES: readonly TOrderState[] = ['new', 'assembling'];

/** Whether an order's deadline still matters: only while the seller has it to hand over (new or assembling). */
export function deadlineMatters(state: TOrderState): boolean {
    return OPEN_STATES.includes(state);
}

/**
 * The tone of an order's deadline: `normal` while there is time, `attention` from 6 hours before it, `overdue` from the deadline
 * on, and `none` for an order with no deadline (an FBO order) or one already handed over or cancelled.
 */
export function deadlineTone(
    deadlineAt: string | undefined,
    state: TOrderState,
    now: Date
): 'none' | 'normal' | 'attention' | 'overdue' {
    if (deadlineAt === undefined || !deadlineMatters(state)) return 'none';
    const left = new Date(deadlineAt).getTime() - now.getTime();
    if (left <= 0) return 'overdue';
    return left <= DEADLINE_ATTENTION_HOURS * 3_600_000 ? 'attention' : 'normal';
}
