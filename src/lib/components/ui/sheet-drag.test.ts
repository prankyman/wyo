import { describe, expect, it } from 'vitest';
import { sheetDragOutcome } from './sheet-drag';

// A sheet 600 px high: a quarter of it is 150 px.
const SHEET = 600;

describe('sheetDragOutcome (the bottom sheet’s grab handle)', () => {
    it.each([
        ['a tap closes', 0, 120, 'close'],
        ['a tap that wobbled a few px still closes', 5, 200, 'close'],
        ['a long press that did not move stays', 2, 800, 'stay'],
        ['a slow drag past a quarter of the sheet closes', 150, 1500, 'close'],
        ['a slow drag short of a quarter springs back', 140, 1500, 'stay'],
        ['a quick flick closes from a short distance', 60, 100, 'close'],
        ['a flick shorter than 24 px springs back', 20, 20, 'stay']
    ] as const)('%s', (_case, distance, durationMs, outcome) => {
        expect(sheetDragOutcome({ distance, durationMs, sheetHeight: SHEET })).toBe(outcome);
    });
});
