import { goto, preloadData, pushState } from '$app/navigation';
import { page } from '$app/state';

/**
 * A row's details in a side panel without a page reload (SvelteKit "shallow routing":
 * https://svelte.dev/docs/kit/shallow-routing). The row keeps its ordinary link (`?item=<id>`), so a middle click, a copied link
 * and a page without scripts still work; a plain press is taken over here: the address changes and the browser's history gains
 * an entry, but the page is not loaded again and the list does not move. The details come from the data the page's own load
 * returns for that address (`preloadData`), and are kept in `page.state` until the entry goes away.
 */

interface IShallowOptions {
    /** The query parameter that names the opened record (`item`, `edit`). */
    param: string;
    /** What the page's load returned for the opened address: the record's details, or nothing when it does not exist. */
    pick: (data: Record<string, unknown>) => unknown;
}

/** True while a press is still being opened, so a second press does not push a second entry. */
let opening = false;

/** The link a press landed on, when it is a plain left press on a same-page link that carries `param`; otherwise null. */
function shallowTarget(event: MouseEvent, param: string): URL | null {
    if (event.defaultPrevented || event.button !== 0) return null;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return null;
    const anchor = event.target instanceof Element ? event.target.closest('a[href]') : null;
    if (!anchor) return null;
    const target = anchor.getAttribute('target');
    if (anchor.hasAttribute('download') || (target !== null && target !== '' && target !== '_self')) return null;
    const url = new URL(anchor.getAttribute('href') ?? '', page.url);
    if (url.origin !== page.url.origin || url.pathname !== page.url.pathname) return null;
    return url.searchParams.has(param) ? url : null;
}

/**
 * Handler for the click (in the capture phase, `<svelte:window onclickcapture>`): opens the record in a shallow history entry.
 * When the details cannot be loaded this way, the press is followed as the ordinary navigation it would have been.
 */
export async function openShallowLink(event: MouseEvent, options: IShallowOptions): Promise<void> {
    const url = shallowTarget(event, options.param);
    if (!url) return;
    event.preventDefault();
    if (opening) return;
    opening = true;
    const href = url.pathname + url.search;
    try {
        const result = await preloadData(href);
        const details = result.type === 'loaded' && result.status === 200 ? options.pick(result.data) : null;
        if (details) {
            pushState(href, {
                details: { param: options.param, id: url.searchParams.get(options.param) ?? '', value: details }
            });
        } else {
            await goto(href, { noScroll: true });
        }
    } finally {
        opening = false;
    }
}

/**
 * Closes the details. Opened by a press, the history has an entry of ours: going back removes `?<param>=` and returns to the same
 * place in the list. Opened by a direct address (a reload, a link), there is no such entry, so the list's own address replaces it.
 */
export function closeShallowDetails(param: string, listHref: string): void {
    if (page.state.details?.param === param) {
        history.back();
        return;
    }
    void goto(listHref, { noScroll: true, keepFocus: true, replaceState: true });
}
