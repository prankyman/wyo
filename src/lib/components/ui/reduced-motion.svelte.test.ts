import { flushSync } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createReducedMotion, prefersReducedMotion } from './reduced-motion.svelte';

/** A `matchMedia` that answers `matches` and keeps the listeners it was given, so a test can change the answer. */
function stubMatchMedia(matches: boolean) {
    const listeners = new Set<(event: MediaQueryListEvent) => void>();
    const query = {
        matches,
        addEventListener: vi.fn((_type: string, listener: (event: MediaQueryListEvent) => void) =>
            listeners.add(listener)
        ),
        removeEventListener: vi.fn((_type: string, listener: (event: MediaQueryListEvent) => void) =>
            listeners.delete(listener)
        )
    };
    const matchMedia = vi.fn(() => query as unknown as MediaQueryList);
    vi.stubGlobal('matchMedia', matchMedia);
    return { query, matchMedia, listeners };
}

afterEach(() => vi.unstubAllGlobals());

describe('prefersReducedMotion', () => {
    it.each([
        [true, true],
        [false, false]
    ])('answers %s when the visitor asked for less motion: %s', (matches, expected) => {
        stubMatchMedia(matches);

        expect(prefersReducedMotion()).toBe(expected);
    });

    it('answers false where the browser has no matchMedia', () => {
        vi.stubGlobal('matchMedia', undefined);

        expect(prefersReducedMotion()).toBe(false);
    });
});

describe('createReducedMotion', () => {
    it('reads the answer at once, follows a change of it, and stops listening when its owner goes', () => {
        const { query, matchMedia, listeners } = stubMatchMedia(false);
        let motion!: { readonly current: boolean };
        const stop = $effect.root(() => {
            motion = createReducedMotion();
        });
        flushSync();

        expect(matchMedia).toHaveBeenCalledWith('(prefers-reduced-motion: reduce)');
        expect(motion.current).toBe(false);

        for (const listener of listeners) listener({ matches: true } as MediaQueryListEvent);
        expect(motion.current).toBe(true);

        stop();
        expect(query.removeEventListener).toHaveBeenCalledTimes(1);
        expect(listeners.size).toBe(0);
    });

    it('reports false where the browser has no matchMedia', () => {
        vi.stubGlobal('matchMedia', undefined);
        let motion!: { readonly current: boolean };
        const stop = $effect.root(() => {
            motion = createReducedMotion();
        });
        flushSync();

        expect(motion.current).toBe(false);
        stop();
    });
});
