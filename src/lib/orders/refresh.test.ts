import { describe, expect, it } from 'vitest';
import { minutesUntil } from './refresh';

describe('the minutes until «Обновить» may be pressed again', () => {
    it.each([
        ['no time is given', undefined, 0],
        ['the time has passed', '2026-11-18T11:59:00', 0],
        ['it is the very time', '2026-11-18T12:00:00', 0],
        ['10 seconds are left, rounded up to a whole minute', '2026-11-18T12:00:10', 1],
        ['exactly 4 minutes are left', '2026-11-18T12:04:00', 4],
        ['4 minutes and a second are left', '2026-11-18T12:04:01', 5]
    ])('is right when %s', (_case, next, minutes) => {
        expect(minutesUntil(next, new Date('2026-11-18T12:00:00'))).toBe(minutes);
    });
});
