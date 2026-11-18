import { describe, expect, it } from 'vitest';
import { journalFilterSchema, orderFilterSchema, readListFilter } from './list-filters';

describe('the filters of the order list and the journal', () => {
    it('reads the filters from the address, every one empty when it is not there', async () => {
        const url = new URL('http://localhost/orders?account=7&state=new&q=DRESS&from=2026-11-01');

        await expect(readListFilter(url, orderFilterSchema, 'orders-filter')).resolves.toEqual({
            account: '7',
            scheme: '',
            state: 'new',
            from: '2026-11-01',
            to: '',
            q: 'DRESS',
            attention: ''
        });
    });

    it('reads «Требуют внимания» and the place a journal request went to from the address', async () => {
        await expect(
            readListFilter(new URL('http://localhost/orders?attention=true'), orderFilterSchema, 'orders-filter')
        ).resolves.toMatchObject({ attention: 'true' });
        await expect(
            readListFilter(new URL('http://localhost/journal?target=fulfilment'), journalFilterSchema, 'journal-filter')
        ).resolves.toMatchObject({ target: 'fulfilment' });
    });

    it('shows the whole list for an attention filter that is not «true» or empty', async () => {
        await expect(
            readListFilter(new URL('http://localhost/orders?attention=maybe'), orderFilterSchema, 'orders-filter')
        ).resolves.toMatchObject({ attention: '' });
    });

    it('drops an address it cannot read and shows the whole list instead of a refusal', async () => {
        const url = new URL('http://localhost/journal?outcome=bogus&account=7');

        await expect(readListFilter(url, journalFilterSchema, 'journal-filter')).resolves.toEqual({
            account: '',
            outcome: '',
            target: '',
            from: '',
            to: ''
        });
    });

    it('refuses a period that ends before it begins, under the last day', () => {
        const result = orderFilterSchema.safeParse({ from: '2026-11-10', to: '2026-11-01' });

        expect(result.error?.issues.map((issue) => [issue.path.join('.'), issue.message])).toEqual([
            ['to', 'errors.period_reversed']
        ]);
    });
});
