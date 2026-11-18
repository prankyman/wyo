import { screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { accountOf, MARKETPLACE, MARKETPLACE_NAME, orderOf } from '$test/support/marketplaces';
import { renderPageWith } from '$test/support/render-page';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import type { TOrder, TOrderDetails } from '$lib/components/orders/types';
import type { TOrderFilter } from '$lib/validation/list-filters';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

import Page from './+page.svelte';

const NO_FILTER: TOrderFilter = { account: '', scheme: '', state: '', from: '', to: '', q: '' };

/** The data the page's load hands it; each test changes only what it is about. */
function pageData(
    change: {
        filter?: Partial<TOrderFilter>;
        orders?: TOrder[];
        accounts?: ReturnType<typeof accountOf>[];
        opened?: TOrderDetails | null;
        refusal?: string | null;
        total?: number;
    } = {}
) {
    const orders = change.orders ?? [orderOf(), orderOf({ id: 102, number: '4455667799', scheme: 'fbo' })];
    return {
        filter: { ...NO_FILTER, ...change.filter },
        list: change.refusal ? null : { orders, total: change.total ?? orders.length, page: 1, pageSize: 50 },
        refusal: change.refusal ?? null,
        accounts: change.accounts ?? [accountOf()],
        marketplaces: [MARKETPLACE],
        refresh: {},
        opened: change.opened ?? null
    };
}

beforeEach(() => setPageUrl('/orders'));
afterEach(() => resetPage());

describe('«Заказы»', () => {
    it('draws its heading with one «?» and «Обновить», the filter folded, a row for each order linked to its details, and the pages', () => {
        renderPageWith(Page, pageData({ total: 120 }));

        expect(screen.getByTestId('orders-heading')).toHaveTextContent('Заказы');
        expect(screen.getAllByTestId('orders-help-button')).toHaveLength(1);
        expect(screen.getByTestId('orders-refresh')).toHaveTextContent('Обновить');
        expect((screen.getByTestId('orders-filter') as HTMLDetailsElement).open).toBe(false);
        const table = screen.getByTestId('orders-list-table');
        const rows = within(table).getAllByTestId('orders-list-row');
        expect(rows.map((row) => row.getAttribute('data-order-id'))).toEqual(['101', '102']);
        expect(within(rows[0]).getByTestId('orders-list-link')).toHaveAttribute('href', '/orders?order=101');
        expect(within(rows[0]).getByTestId('orders-list-mark')).toHaveTextContent(MARKETPLACE_NAME);
        expect(rows[0]).toHaveTextContent('Магазин одежды');
        expect(rows[0]).toHaveTextContent('Платья · Северный ветер');
        expect(within(rows[0]).getByTestId('orders-list-state')).toHaveTextContent('Новый');
        expect(within(table).getAllByTestId('orders-list-number-help-button')).toHaveLength(1);
        expect(screen.getByTestId('orders-pagination')).toHaveTextContent('Показано 1–50 из 120');
    });

    it('teaches what to do when no account exists: a sentence and the way to «Маркетплейсы», and no filter', () => {
        renderPageWith(Page, pageData({ accounts: [], orders: [] }));

        expect(screen.getByTestId('orders-empty-no-accounts')).toHaveTextContent('не добавлен ни один аккаунт');
        expect(screen.getByTestId('orders-open-marketplaces')).toHaveAttribute('href', '/marketplaces');
        expect(screen.queryByTestId('orders-filter')).toBeNull();
        expect(screen.getByTestId('orders-refresh')).toBeDisabled();
    });

    it('says when there are no orders yet and «Обновить» reads them', () => {
        renderPageWith(Page, pageData({ orders: [] }));

        expect(screen.getByTestId('orders-empty')).toHaveTextContent('нажмите «Обновить»');
        expect(screen.queryByTestId('orders-list-table')).toBeNull();
    });

    it('shows the filters in use as chips linked to the address without each, and says what was filtered when nothing is found', () => {
        setPageUrl('/orders?account=7&state=new&q=DRESS');
        renderPageWith(Page, pageData({ filter: { account: '7', state: 'new', q: 'DRESS' }, orders: [] }));

        expect(screen.getByTestId('orders-filter-chip-account')).toHaveAttribute('href', '/orders?state=new&q=DRESS');
        expect(screen.getByTestId('orders-filter-chip-state')).toHaveTextContent('Состояние: Новый');
        expect(screen.getByTestId('orders-filter-reset')).toHaveAttribute('href', '/orders');
        expect(screen.getByTestId('orders-filter-account')).toHaveValue('7');
        expect(screen.getByTestId('orders-empty-filtered')).toHaveTextContent(
            'По фильтру ничего не найдено: «Аккаунт: Магазин одежды», «Состояние: Новый», «Поиск: DRESS».'
        );
    });

    it('opens the details of the order the address names beside the list', () => {
        setPageUrl('/orders?order=101');
        renderPageWith(Page, pageData({ opened: { order: orderOf(), events: [] } }));

        expect(screen.getByTestId('order-details-panel')).toHaveTextContent('Заказ 4455667788');
        expect(screen.getByTestId('order-details')).toBeInTheDocument();
    });

    it('words the refusal when the list cannot be read', () => {
        renderPageWith(Page, pageData({ refusal: 'internal' }));

        expect(screen.getByTestId('orders-refusal')).toHaveTextContent('В программе wyo произошла ошибка.');
        expect(screen.getByTestId('orders-retry')).toBeInTheDocument();
    });
});
