import { render } from '@testing-library/svelte';

/**
 * Renders a route's page with the data its `load` would hand it — the one line every page test would otherwise write as a local
 * `renderPage`. The data is typed loosely on purpose: a page test passes the part of the load's answer the page reads, built by the
 * page's own fixture, and the page's `PageData` type names everything the load returns.
 *
 * ```ts
 * renderPageWith(Page, pageData({ canWrite: false }));
 * renderPageWith(Page, pageData(), { form: { message: 'saved' } });
 * ```
 */
export function renderPageWith(Page: Parameters<typeof render>[0], data: unknown, props: Record<string, unknown> = {}) {
    return render(Page, { props: { data, ...props } as never });
}
