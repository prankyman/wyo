import { z } from 'zod';
import type { Infer } from 'sveltekit-superforms';
import { readSearch } from '$lib/forms/search-form';

/**
 * The filters of the order list and the journal. They live in the page's address, so a filtered list can be opened again by its
 * link; the page's load reads them with the same schema the filter form checks them with. Every filter is a text: an empty one
 * filters nothing. The wyo server checks them again.
 */

const DAY = z
    .string()
    .regex(/^(\d{4}-\d{2}-\d{2})?$/, 'errors.invalid')
    .default('');
const ID = z.string().regex(/^\d*$/, 'errors.invalid').default('');

/** The period's two days: the last one may not stand before the first. */
function periodInOrder(values: { from: string; to: string }, context: z.RefinementCtx): void {
    if (values.from !== '' && values.to !== '' && values.to < values.from) {
        context.addIssue({ code: 'custom', path: ['to'], message: 'errors.period_reversed' });
    }
}

export const ORDER_STATES = ['new', 'assembling', 'in_delivery', 'received', 'cancelled', 'unknown'] as const;
export const ORDER_SCHEMES = ['fbs', 'fbo'] as const;
export const JOURNAL_OUTCOMES = ['done', 'limited', 'refused', 'server_error', 'no_connection', 'key_refused'] as const;

/** The filter of «Заказы»: account, scheme, state, the period of creation, and a search by number, article or barcode. */
export const orderFilterSchema = z
    .object({
        account: ID,
        scheme: z.enum(['', ...ORDER_SCHEMES]).default(''),
        state: z.enum(['', ...ORDER_STATES]).default(''),
        from: DAY,
        to: DAY,
        q: z.string().trim().max(100, 'errors.search_too_long').default('')
    })
    .superRefine(periodInOrder);

/** The filter of «Журнал запросов»: account, outcome and the period. */
export const journalFilterSchema = z
    .object({
        account: ID,
        outcome: z.enum(['', ...JOURNAL_OUTCOMES]).default(''),
        from: DAY,
        to: DAY
    })
    .superRefine(periodInOrder);

/** The filter forms' ids: a page's form and its load read the same values. */
export const ORDER_FILTER_ID = 'orders-filter';
export const JOURNAL_FILTER_ID = 'journal-filter';

export type TOrderFilter = z.infer<typeof orderFilterSchema>;
export type TJournalFilter = z.infer<typeof journalFilterSchema>;

/**
 * The filter a list's address holds. An address that cannot be read (a state that does not exist, a period turned round) shows
 * the whole list: the filter form then shows empty boxes, and nothing is refused before the person typed anything.
 */
export async function readListFilter<TSchema extends typeof orderFilterSchema | typeof journalFilterSchema>(
    url: URL,
    schema: TSchema,
    id: string
): Promise<Infer<TSchema, 'zod4'>> {
    const read = await readSearch(url, schema, id);
    if (read.valid) return read.data as Infer<TSchema, 'zod4'>;
    // TSchema is one of two schemas, and TypeScript does not narrow what `parse` returns for such a type, so the empty filter
    // goes through `unknown`; `parse` itself checks the value against the schema it was given.
    return schema.parse({}) as unknown as Infer<TSchema, 'zod4'>;
}
