/**
 * A reactive stand-in for `page` from `$app/state`.
 *
 * The global mock at `src/test/mocks/$app/state.ts` is a frozen plain object: `page.url` is a
 * literal with a `searchParams.get` that always answers `null`, and nothing on it is reactive.
 * That is enough for a component that reads the URL once at mount, and useless for one whose
 * behaviour IS the reaction to a navigation: a guard or a tab sync living in an `$effect` that must re-run
 * when the URL changes. Mutating a plain object cannot re-run an effect, so those were untestable.
 *
 * `url` is a real `URL`, so `pathname`, `search` and `searchParams` all behave as they do in
 * production rather than as whatever subset a hand-rolled literal happened to define.
 *
 * `status` and `data` exist for the root error boundary (`src/routes/+error.svelte`), which is the
 * only component whose entire output is decided by the HTTP status it was rendered for. Everything
 * else leaves them at 200 and `{}`.
 *
 * `navigating` is the reactive stand-in for the in-progress navigation: `setNavigating('/showcase')`
 * starts one — by default a link press; `setNavigating('/showcase', 'goto')` one started by code — and
 * `setNavigating(null)` ends it. The press-feedback components (`Button`, the clickable cards, the
 * form guard, the page-change guard) read it, so every mock of `$app/state` must carry it.
 *
 * Use through `vi.mock`, and reset in `afterEach` so the URL does not leak between tests:
 *
 * ```ts
 * vi.mock('$app/state', async () => {
 *     const state = await import('$test/support/page-state.svelte');
 *     return { page: state.page, navigating: state.navigating };
 * });
 * afterEach(() => resetPage());
 * ```
 */
import type { NavigationType } from '@sveltejs/kit';

const HOST = 'http://localhost';

class ReactivePage {
    url = $state(new URL(HOST + '/'));
    params = $state<Record<string, string>>({});
    route = $state<{ id: string | null }>({ id: null });
    /** Only the root error page reads these two; everything else leaves them at the defaults. */
    status = $state(200);
    data = $state<Record<string, unknown>>({});
    /** The error the root error page renders; null otherwise. */
    error = $state<App.Error | null>(null);
    /** The shallow-history state a `pushState` call wrote (the open bottom sheet of a `Combobox` on a touch screen); empty otherwise. */
    state = $state<App.PageState>({});
}

export const page = new ReactivePage();

/** The part of SvelteKit's `Navigation` the application reads: where it is going. */
interface INavigationTarget {
    url: URL;
}

class ReactiveNavigating {
    to = $state<INavigationTarget | null>(null);
    /** What started it: a link, a GET form, `goto`, the back/forward buttons. */
    type = $state<Exclude<NavigationType, 'enter'> | null>(null);
}

export const navigating = new ReactiveNavigating();

/**
 * Start a navigation to `href` (a path or a full URL), or end it with `null`. `type` is what
 * started it; a link press unless the test says otherwise.
 */
export function setNavigating(href: string | null, type: Exclude<NavigationType, 'enter'> = 'link'): void {
    navigating.to = href === null ? null : { url: new URL(href, HOST) };
    navigating.type = href === null ? null : type;
}

/** Navigate the fixture. Accepts a path (`/showcase?tab=forms`) or a full URL. */
export function setPageUrl(href: string): void {
    page.url = new URL(href, HOST);
}

/** The HTTP status the root error boundary is rendering for. */
export function setPageStatus(status: number): void {
    page.status = status;
}

/** The merged branch data — for an error page, the root layout's `locale`/`defaultLocale`. */
export function setPageData(data: Record<string, unknown>): void {
    page.data = data;
}

/** The error the root error boundary is rendering for. */
export function setPageError(error: App.Error | null): void {
    page.error = error;
}

export function resetPage(): void {
    page.url = new URL(HOST + '/');
    page.params = {};
    page.route = { id: null };
    page.status = 200;
    page.data = {};
    page.error = null;
    page.state = {};
    navigating.to = null;
    navigating.type = null;
}
