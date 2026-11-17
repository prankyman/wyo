import { describe, expect, it } from 'vitest';
import { LINK_TARGET, buttonClasses } from './press';

/**
 * The 44 × 44 px tap bar (WCAG 2.5.5): the class strings every screen
 * builds its tap boxes from. They are plain strings, so the test reads them as class lists — the box they
 * draw is measured in a real browser by the phone sweep, never in jsdom.
 */

const tokens = (classes: string): string[] => classes.split(/\s+/).filter(Boolean);

describe('the tap box classes (press.ts)', () => {
    it('gives a standalone link a box of at least 44 × 44 px that keeps the link in line with its text', () => {
        expect(LINK_TARGET).toBe('inline-flex min-h-11 min-w-11 items-center');
    });

    it.each([
        ['md', ['min-h-11', 'min-w-11']],
        ['lg', ['min-h-14']],
        ['xl', ['min-h-16']]
    ] as const)(
        'makes a %s text button at least as tall as its size, and a short label still 44 px wide',
        (size, held) => {
            const classes = tokens(buttonClasses({ variant: 'primary', size, fullWidth: false, iconOnly: false }));

            expect(classes).toEqual(expect.arrayContaining([...held]));
        }
    );

    it('does not widen the larger buttons: only md carries the 44 px minimum width', () => {
        for (const size of ['lg', 'xl'] as const) {
            expect(
                tokens(buttonClasses({ variant: 'primary', size, fullWidth: false, iconOnly: false }))
            ).not.toContain('min-w-11');
        }
    });
});
