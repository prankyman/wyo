import { vi } from 'vitest';
import { page } from './page-state.svelte';

/**
 * A stand-in for `$app/navigation` in a component's test, with every function a superform reaches for :
 * a search form (`searchForm`) calls `goto`, and superforms itself registers `beforeNavigate`. A test that mocked
 * only `goto` fails as soon as its screen gains a superform.
 *
 * vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);
 */
export const navigationModule = {
    goto: vi.fn(async () => undefined),
    invalidate: vi.fn(async () => undefined),
    invalidateAll: vi.fn(async () => undefined),
    beforeNavigate: vi.fn(),
    afterNavigate: vi.fn(),
    onNavigate: vi.fn(),
    preloadData: vi.fn(async () => undefined),
    pushState: vi.fn(),
    replaceState: vi.fn()
};

/**
 * `navigationModule` whose `pushState` writes the reactive `page.state` (`page-state.svelte.ts`), as SvelteKit does, so a
 * component that reads `page.state` after pushing a shallow history entry sees its entry. Pair it with
 * `stubHistoryBack()` and mock `$app/state` with the reactive `page`.
 *
 * vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).pageStateNavigationModule);
 */
export const pageStateNavigationModule = {
    ...navigationModule,
    pushState: vi.fn((_url: string, state: App.PageState) => {
        page.state = state;
    })
};

/**
 * Replace `history.back()` with a spy that takes the shallow history entry away at once — `page.state` empty and a `popstate`
 * event sent, as the phone's «назад» leaves it. Restore the spy after the test (`spy.mockRestore()`).
 */
export function stubHistoryBack() {
    return vi.spyOn(history, 'back').mockImplementation(() => {
        page.state = {};
        window.dispatchEvent(new PopStateEvent('popstate'));
    });
}
