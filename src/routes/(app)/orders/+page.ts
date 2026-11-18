import { api } from '$lib/api/api';
import type { TAccountList, TMarketplaceList } from '$lib/components/marketplaces/types';
import type { TOrderDetails, TOrderList, TRefreshState } from '$lib/components/orders/types';
import { apiListPath, pageNumber } from '$lib/lists/list-address';
import { ORDER_FILTER_ID, orderFilterSchema, readListFilter } from '$lib/validation/list-filters';
import type { PageLoad } from './$types';

/**
 * «Заказы» reads what its address names: the page of orders with the filters in use (`GET /api/v1/orders`), the accounts and the
 * adapters (for the filter, the marks and the empty state), the state of «Обновить» (`GET /api/v1/sync/refresh`), and the order
 * whose details are open beside the list (`?order=<id>`, `GET /api/v1/orders/{id}`). A list that cannot be read leaves its
 * refusal's code for the page to word.
 */
export const load: PageLoad = async ({ url }) => {
    const filter = await readListFilter(url, orderFilterSchema, ORDER_FILTER_ID);
    const opened = url.searchParams.get('order');
    const [list, accounts, marketplaces, refresh, details] = await Promise.all([
        api.get<TOrderList>(apiListPath('/api/v1/orders', { ...filter, page: pageNumber(url) })),
        api.get<TAccountList>('/api/v1/accounts'),
        api.get<TMarketplaceList>('/api/v1/marketplaces'),
        api.get<TRefreshState>('/api/v1/sync/refresh'),
        opened && /^\d+$/.test(opened) ? api.get<TOrderDetails>(`/api/v1/orders/${opened}`) : null
    ]);
    return {
        filter,
        list: list.ok ? list.data : null,
        refusal: list.ok ? null : list.refusal.code,
        accounts: accounts.ok ? accounts.data.accounts : [],
        marketplaces: marketplaces.ok ? marketplaces.data.marketplaces : [],
        refresh: refresh.ok ? refresh.data : {},
        opened: details?.ok ? details.data : null
    };
};
