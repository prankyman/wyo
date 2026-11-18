import type { IFilterChip } from '$lib/components/ui/types';

/**
 * The address of a list whose filters, page and opened row live in the address, so a filtered list can be opened again by its
 * link and the back button returns to it: the order list and the journal. One set of functions for both.
 */

/** The parameters that belong to one view of a list and are dropped when the filters change: the page and the opened row. */
const VIEW_PARAMS = ['page', 'order', 'entry'];

/**
 * The list's address with `values` set (a `null` or empty value removed). The page number and the opened row are dropped, so a
 * changed filter starts the list again, unless they are named in `keep`; page 1 is written as no page at all.
 */
export function addressWith(
    current: URL,
    values: Record<string, string | null>,
    { keep = [] }: { keep?: readonly string[] } = {}
): string {
    const next = new URL(current);
    for (const name of VIEW_PARAMS) if (!keep.includes(name) && !(name in values)) next.searchParams.delete(name);
    for (const [name, value] of Object.entries(values)) {
        if (value === null || value === '' || (name === 'page' && value === '1')) next.searchParams.delete(name);
        else next.searchParams.set(name, value);
    }
    return `${next.pathname}${next.search}`;
}

/** The wyo server's call of a list with the filters in use: an empty filter is left out, and so is page 1. */
export function apiListPath(path: string, values: Record<string, string | number>): string {
    const query = new URLSearchParams();
    for (const [name, value] of Object.entries(values)) {
        if (value === '' || (name === 'page' && value === 1)) continue;
        query.set(name, String(value));
    }
    const search = query.toString();
    return search === '' ? path : `${path}?${search}`;
}

/** The chips of the filters in use: one per filter with words (`label` not null), each linked to the address without it. */
export function filterChips(current: URL, filters: readonly { key: string; label: string | null }[]): IFilterChip[] {
    return filters.flatMap(({ key, label }) =>
        label === null ? [] : [{ key, label, href: addressWith(current, { [key]: null }) }]
    );
}

/** The page number the address holds: a whole number from 1; the first page when the address holds none or a wrong one. */
export function pageNumber(url: URL): number {
    const text = url.searchParams.get('page') ?? '';
    return /^[1-9]\d*$/.test(text) ? Number(text) : 1;
}
