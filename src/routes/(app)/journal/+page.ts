import { api } from '$lib/api/api';
import type { TJournalList } from '$lib/components/journal/types';
import type { TAccountList, TMarketplaceList } from '$lib/components/marketplaces/types';
import { apiListPath, pageNumber } from '$lib/lists/list-address';
import { JOURNAL_FILTER_ID, journalFilterSchema, readListFilter } from '$lib/validation/list-filters';
import type { PageLoad } from './$types';

/**
 * «Журнал запросов» reads what its address names: the page of the journal with the filters in use (`GET /api/v1/journal`), and
 * the accounts and adapters for the filter and the marks. The row open beside the list (`?entry=<id>`) is taken from the page just
 * read: the journal has no call for one row. A journal that cannot be read leaves its refusal's code for the page to word.
 */
export const load: PageLoad = async ({ url }) => {
    const filter = await readListFilter(url, journalFilterSchema, JOURNAL_FILTER_ID);
    const [list, accounts, marketplaces] = await Promise.all([
        api.get<TJournalList>(apiListPath('/api/v1/journal', { ...filter, page: pageNumber(url) })),
        api.get<TAccountList>('/api/v1/accounts'),
        api.get<TMarketplaceList>('/api/v1/marketplaces')
    ]);
    const entry = url.searchParams.get('entry');
    return {
        filter,
        list: list.ok ? list.data : null,
        refusal: list.ok ? null : list.refusal.code,
        accounts: accounts.ok ? accounts.data.accounts : [],
        marketplaces: marketplaces.ok ? marketplaces.data.marketplaces : [],
        opened: (list.ok && list.data.entries.find((one) => String(one.id) === entry)) || null
    };
};
