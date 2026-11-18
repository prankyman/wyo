import { describe, expect, it } from 'vitest';
import { deadlineMatters, deadlineTone, DEADLINE_ATTENTION_HOURS } from './deadline';

const DEADLINE = '2026-11-19T12:00:00';
const at = (time: string) => new Date(time);

describe('the tone of an order’s deadline', () => {
    it('warns 6 hours before the deadline, the number the order list promises', () => {
        expect(DEADLINE_ATTENTION_HOURS).toBe(6);
    });

    it.each([
        ['a day before', '2026-11-18T12:00:00', 'normal'],
        ['6 hours and a minute before', '2026-11-19T05:59:00', 'normal'],
        ['exactly 6 hours before', '2026-11-19T06:00:00', 'attention'],
        ['a minute before', '2026-11-19T11:59:00', 'attention'],
        ['at the deadline', '2026-11-19T12:00:00', 'overdue'],
        ['a day after', '2026-11-20T12:00:00', 'overdue']
    ] as const)('is %s: %s', (_when, now, tone) => {
        expect(deadlineTone(DEADLINE, 'new', at(now))).toBe(tone);
    });

    it.each(['in_delivery', 'received', 'cancelled', 'unknown'] as const)(
        'is quiet for an order already %s, since nothing is left to hand over',
        (state) => {
            expect(deadlineTone(DEADLINE, state, at('2026-11-20T12:00:00'))).toBe('none');
        }
    );

    it('is quiet for an order with no deadline (an FBO order)', () => {
        expect(deadlineTone(undefined, 'new', at('2026-11-20T12:00:00'))).toBe('none');
    });

    it.each([
        ['new', true],
        ['assembling', true],
        ['in_delivery', false],
        ['received', false],
        ['cancelled', false],
        ['unknown', false]
    ] as const)('matters for an order %s: %s, only while it waits to be handed over', (state, matters) => {
        expect(deadlineMatters(state)).toBe(matters);
    });
});
