import { browser } from '$app/environment';

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

/**
 * Whether this visitor has asked for less motion at this moment. A plain answer for code that is not a
 * component and so cannot hold the rune below. Same rule, same `false` where there is no `window.matchMedia`.
 */
export function prefersReducedMotion(): boolean {
    if (!browser || typeof window.matchMedia === 'undefined') return false;
    return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

/**
 * Whether this visitor has asked their operating system for less motion.
 *
 * One rule, one home: a panel that slides, a ring that pulses or a scroll that animates all
 * ask this before moving anything. The rule is not decorative — motion nobody asked for is
 * a barrier for people who get migraines or vertigo from it.
 *
 * Returns a getter rather than a value because a rune's reactivity does not survive being
 * returned from a function. Call it in a component's initialisation, read `.current` where
 * the answer is needed.
 *
 * Reports `false` where `window.matchMedia` is absent — server-side rendering, and jsdom in
 * the test suite, which is what lets components using this render under both.
 */
export function createReducedMotion(): { readonly current: boolean } {
    let reduced = $state(false);

    $effect(() => {
        if (!browser || typeof window.matchMedia === 'undefined') {
            reduced = false;
            return;
        }
        const query = window.matchMedia(REDUCED_MOTION_QUERY);
        reduced = query.matches;
        const onChange = (event: MediaQueryListEvent) => {
            reduced = event.matches;
        };
        query.addEventListener('change', onChange);
        return () => query.removeEventListener('change', onChange);
    });

    return {
        get current() {
            return reduced;
        }
    };
}
