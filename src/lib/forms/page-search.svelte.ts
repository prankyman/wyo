import { page } from '$app/state';
import { untrack } from 'svelte';
import type { Infer, SuperForm } from 'sveltekit-superforms';
import type { z } from 'zod';
import { searchForm } from './search-form';

/**
 * A list screen's search or filter form: a `searchForm` on the page's own address whose boxes
 * follow what the list shows — after the back button, or a link that changes the filter, the boxes show the address's
 * values again. `values` reads them from the page's data; the parameter names are the address's own.
 *
 * The boxes are put back only when the ADDRESS changes: a live list re-reads its data on
 * every notice without changing the address, and a choice made in a box but not sent yet must survive that re-read.
 *
 * Call it while the component starts, like `superForm`.
 */
export function pageSearch<TSchema extends z.ZodType<Record<string, unknown>>>(
    schema: TSchema,
    values: () => Infer<TSchema, 'zod4'>,
    id: string
): SuperForm<Infer<TSchema, 'zod4'>> {
    const form = searchForm(schema, values(), { id, address: () => page.url });
    let shown = addressOf(page.url);
    $effect(() => {
        const address = addressOf(page.url);
        if (address === shown) return;
        shown = address;
        const next = untrack(values);
        form.form.update(() => next, { taint: false });
    });
    return form;
}

/** The part of the address a list's filter lives in. */
function addressOf(url: URL): string {
    return `${url.pathname}${url.search}`;
}
