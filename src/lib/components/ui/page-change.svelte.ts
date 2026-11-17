import { navigating } from '$app/state';

declare global {
    interface Window {
        /**
         * The start-up submission guard of `src/app.html`: it stops every form submission until the interface has started
         * (static files have nothing to post to). `release()` removes it and says whether a submission was stopped.
         */
        __startupSubmitGuard?: { release(): boolean };
        /**
         * The start-up page-change bar of `src/app.html`: shows the running bar for a link pressed before the interface has
         * started. `release()` stops it watching presses.
         */
        __startupPageChangeBar?: { release(): void };
    }
}

/**
 * "The page is being replaced" — the one fact the page-change guard, the form guard, `Button` and
 * the clickable cards all read.
 *
 * A page changes in one of two ways:
 *
 * 1. SvelteKit navigates in the browser (a link, `goto`, a GET form, a redirect after an enhanced
 *    form). `navigating.to` from `$app/state` is set until the new page is on screen, and is set
 *    back to `null` on every ending — success, error page, or a newer navigation taking over.
 * 2. The browser itself loads the answer of a plain POST form (no `use:enhance`). SvelteKit knows
 *    nothing about that, so the form guard records it here. The page is gone when the answer
 *    arrives; the only way back to this same page is the back/forward cache, where `pageshow`
 *    clears it (`PageChangeGuard` listens).
 *
 * Only a change the person started by pressing locks the page (`locking`): a link, a GET form, a
 * plain form. A `goto` from code does not — a search box that navigates on every
 * pause in typing would, on a locked page, drop the letters typed meanwhile and the focus with
 * them — and neither do the back/forward buttons. A button whose own action calls `goto` shows
 * that it is working and refuses a second press itself (`Pagination`, `Tabs`).
 *
 * `$app/state` is read nowhere else in the press-feedback components, so a test that mocks it needs
 * `navigating` next to `page` (see `$test/support/page-state.svelte`).
 */

/** What SvelteKit reports as the cause of a navigation a person started by pressing (`navigating.type`). */
const PRESSED_NAVIGATIONS: ReadonlySet<string> = new Set(['link', 'form']);

/**
 * How long a page change may run before it is taken as dropped: SvelteKit does not always clear
 * `navigating` when a newer re-read takes a navigation over (`client.js`, "navigation aborted"), so
 * a change still "running" after this long gives the page back — the page-change guard unlocks, the
 * links stop saying "opening" and swallowing taps, and a live re-read no longer waits for it.
 */
export const LOCK_MAX_MS = 10_000;

class PageChangeState {
    /** A plain form's answer is being loaded by the browser as a whole new page. */
    nativeSubmission = $state(false);

    /**
     * The change that is running has been running for `LOCK_MAX_MS`. Set and cleared by
     * `PageChangeGuard`, which times it; `isNavigatingTo` reads it, so a link stops showing
     * "opening" and swallowing taps once the page itself is given back.
     */
    lockExpired = $state(false);

    /** Where SvelteKit is going, or `null` when it is not navigating. */
    get destination(): URL | null {
        return navigating.to?.url ?? null;
    }

    /**
     * A page change the person started by pressing is under way, so the page takes no further
     * taps: a plain form's answer is loading, or SvelteKit is following a link or a GET form.
     */
    get locking(): boolean {
        return this.nativeSubmission || (this.destination !== null && PRESSED_NAVIGATIONS.has(navigating.type ?? ''));
    }
}

export const pageChange = new PageChangeState();

/**
 * Whether SvelteKit is on its way to `href` right now — the link that was pressed shows it is
 * working. Compared on path and query; the `#fragment` is not a different page. Once the page
 * has been given back (`lockExpired`) the change is taken as dropped and no link is "opening".
 */
export function isNavigatingTo(href: string | undefined): boolean {
    const destination = pageChange.destination;
    if (!href || !destination || pageChange.lockExpired) return false;
    const target = new URL(href, window.location.href);
    return target.pathname === destination.pathname && target.search === destination.search;
}
