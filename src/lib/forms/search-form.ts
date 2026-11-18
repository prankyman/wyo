import { goto } from '$app/navigation';
import type { Infer, InferIn, SuperForm, SuperValidated } from 'sveltekit-superforms';
import type { z } from 'zod';
import { defaults, superForm, superValidate, zod4, zod4Client } from './super-form';

/**
 * A search form: a form that reads data instead of saving it — filters, a search box. Its values
 * live in the page's address, so a list can be linked and the back button returns to it.
 *
 * It is a superForm in SPA mode (https://superforms.rocks/concepts/spa): nothing is posted; the zod
 * schema checks the boxes, a refused value shows its message under its box and nothing happens, and a valid submit
 * becomes a page change by SvelteKit's router (`goto` with the values in the address, keeping the focus and the
 * scroll), which re-runs the page's load and redraws the list without a reload. The boxes keep what was typed.
 * The form stays `method="GET"` in its markup, so before the page's script has started it is an ordinary GET form;
 * once it has, superforms takes it over (it switches a GET form in SPA mode to its own handling). Its press stays
 * guarded until `goto` has settled: superforms holds `submitting` until `onUpdate` has finished.
 *
 * The page's load (in the browser: the wyo interface has no server side) reads the same values back with `readSearch`.
 */
export function searchForm<TSchema extends z.ZodType<Record<string, unknown>>>(
    schema: TSchema,
    initial: Infer<TSchema, 'zod4'>,
    { id, address }: { id: string; address: () => URL }
): SuperForm<Infer<TSchema, 'zod4'>> {
    const validators = zod4Client(schema);
    // The project's `superForm` is always in SPA mode. `goto` re-runs the load itself; re-reading the page as well would run it
    // twice. The project's options leave the library's `invalidateAll` out of their type, so it is passed past the type here.
    return superForm(defaults(initial, validators, { id }), {
        validators,
        resetForm: false,
        ...({ invalidateAll: false } as object),
        onUpdate: async ({ form }) => {
            if (!form.valid) return;
            await goto(searchAddress(address(), form.data), { keepFocus: true, noScroll: true });
        }
    });
}

/**
 * The address a search lands on: the current one with the form's values set, an empty value removed so the address
 * holds only what narrows the list, and the page number dropped — a new search starts on its first page.
 */
export function searchAddress(current: URL, values: Record<string, unknown>): string {
    const next = new URL(current);
    next.searchParams.delete('page');
    for (const [name, value] of Object.entries(values)) {
        next.searchParams.delete(name);
        const list = Array.isArray(value) ? value : [value];
        for (const item of list) {
            if (item === undefined || item === null || item === '' || item === false) continue;
            next.searchParams.append(name, String(item));
        }
    }
    return `${next.pathname}${next.search}${next.hash}`;
}

/** The page's load reads a search form's values from the address through the same schema. */
export function readSearch<TSchema extends z.ZodType<Record<string, unknown>>>(
    url: URL,
    schema: TSchema,
    id: string
): Promise<SuperValidated<Infer<TSchema, 'zod4'>, unknown, InferIn<TSchema, 'zod4'>>> {
    return superValidate(url, zod4(schema), { id });
}
