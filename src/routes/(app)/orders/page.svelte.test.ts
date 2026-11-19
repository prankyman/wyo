import { screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
    accountOf,
    lineOf,
    MARKETPLACE,
    MARKETPLACE_NAME,
    orderDetailsOf,
    orderOf,
    SECOND_MARKETPLACE,
    SECOND_MARKETPLACE_CODE
} from '$test/support/marketplaces';
import { renderPageWith } from '$test/support/render-page';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import type { TMarketplace } from '$lib/components/marketplaces/types';
import type { TOrder, TOrderDetails } from '$lib/components/orders/types';
import type { TOrderFilter } from '$lib/validation/list-filters';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

import Page from './+page.svelte';

const NO_FILTER: TOrderFilter = {
    account: '',
    scheme: '',
    state: '',
    from: '',
    to: '',
    q: '',
    attention: '',
    test: ''
};

/** The data the page's load hands it; each test changes only what it is about. */
function pageData(
    change: {
        filter?: Partial<TOrderFilter>;
        orders?: TOrder[];
        accounts?: ReturnType<typeof accountOf>[];
        marketplaces?: TMarketplace[];
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
        marketplaces: change.marketplaces ?? [MARKETPLACE],
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

    it('names the parts of a split order under its state, and no parts under an order that was not split', () => {
        renderPageWith(
            Page,
            pageData({
                orders: [
                    orderOf({ id: 101, number: '70-1', state: 'cancelled', parts: ['70-2', '70-3'] }),
                    orderOf({ id: 102, number: '70-2', parentNumber: '70-1' })
                ]
            })
        );

        const rows = within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-row');
        expect(within(rows[0]).getByTestId('orders-list-state')).toHaveTextContent('Отменён');
        expect(within(rows[0]).getByTestId('orders-list-state-parts')).toHaveTextContent(
            'Разделён на части: 70-2, 70-3'
        );
        expect(within(rows[1]).queryByTestId('orders-list-state-parts')).toBeNull();
    });

    it('draws the product of each order with the product cell: the first product, its quantity, and the count of the rest', () => {
        renderPageWith(
            Page,
            pageData({
                orders: [
                    orderOf(),
                    orderOf({
                        id: 103,
                        number: '4455667800',
                        lines: [
                            lineOf({ quantity: 2 }),
                            lineOf({ article: 'SHIRT-BLUE-M' }),
                            lineOf({ article: 'HAT-GREY' })
                        ]
                    })
                ]
            })
        );

        const rows = within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-row');
        expect(within(rows[0]).getByTestId('orders-list-product-first')).toHaveTextContent('артикул DRESS-RED-42');
        expect(within(rows[0]).queryByTestId('orders-list-product-more')).toBeNull();
        expect(within(rows[1]).getByTestId('orders-list-product-quantity')).toHaveTextContent('× 2');
        expect(within(rows[1]).getByTestId('orders-list-product-more')).toHaveTextContent('ещё товаров: 2');
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

    it('draws the exchange’s state in its own column for an FBS order that has one, and nothing for an order that has none', () => {
        renderPageWith(
            Page,
            pageData({
                orders: [
                    orderOf({ exchange: { state: 'held', attention: 'held' } }),
                    orderOf({ id: 102, number: '4455667799', scheme: 'fbo' }),
                    orderOf({ id: 103, number: '4455667800', exchange: { state: 'sent' } })
                ]
            })
        );

        const rows = within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-row');
        expect(within(rows[0]).getByTestId('orders-list-exchange-state')).toHaveTextContent('ждёт у фулфилмент-центра');
        expect(within(rows[1]).queryByTestId('orders-list-exchange-state')).toBeNull();
        expect(within(rows[2]).getByTestId('orders-list-exchange-state')).toHaveTextContent('передан');
        expect(
            within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-exchange-help-button')
        ).toHaveLength(1);
    });

    it('says «не передаётся» in the exchange column for an FBS order of a marketplace that cannot exchange yet, and for no other order', () => {
        const stranger = (change: Partial<TOrder>) => orderOf({ marketplace: SECOND_MARKETPLACE_CODE, ...change });
        renderPageWith(
            Page,
            pageData({
                marketplaces: [MARKETPLACE, SECOND_MARKETPLACE],
                orders: [
                    orderOf(),
                    stranger({ id: 102, number: '89491381-0072-1' }),
                    stranger({ id: 103, number: '89491381-0073-1', scheme: 'fbo' }),
                    stranger({ id: 104, number: '89491381-0074-1', exchange: { state: 'not_sent' } })
                ]
            })
        );

        const rows = within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-row');
        expect(within(rows[0]).queryByTestId('orders-list-exchange-unsupported')).toBeNull();
        expect(within(rows[1]).getByTestId('orders-list-exchange-unsupported')).toHaveTextContent('не передаётся');
        expect(within(rows[1]).getByTestId('orders-list-exchange-unsupported-help-button')).toBeInTheDocument();
        expect(within(rows[2]).queryByTestId('orders-list-exchange-unsupported')).toBeNull();
        // The wyo server gives every FBS order its exchange's state («не передан», «Внешний API» empty): the marketplace's lack
        // of the exchange is the reason shown all the same.
        expect(within(rows[3]).getByTestId('orders-list-exchange-unsupported')).toHaveTextContent('не передаётся');
        expect(within(rows[3]).queryByTestId('orders-list-exchange-state')).toBeNull();
    });

    it('marks a test order beside its scheme in the row, and no other order', () => {
        renderPageWith(
            Page,
            pageData({ orders: [orderOf({ test: true }), orderOf({ id: 102, number: '4455667799' })] })
        );

        const rows = within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-row');
        expect(within(rows[0]).getByTestId('orders-list-test')).toHaveTextContent('Тестовый заказ');
        expect(within(rows[1]).queryByTestId('orders-list-test')).toBeNull();
    });

    it('says «Склад не определён» for an FBS or Express order with no warehouse, and a dash for an order the marketplace packs', () => {
        renderPageWith(
            Page,
            pageData({
                orders: [
                    orderOf({ id: 101, warehouse: undefined }),
                    orderOf({ id: 102, number: '4455667799', scheme: 'express', warehouse: undefined }),
                    orderOf({ id: 103, number: '4455667800', scheme: 'fby', warehouse: undefined }),
                    orderOf({ id: 104, number: '4455667801' })
                ]
            })
        );

        const rows = within(screen.getByTestId('orders-list-table')).getAllByTestId('orders-list-row');
        expect(within(rows[0]).getByTestId('orders-list-warehouse')).toHaveTextContent('Склад не определён');
        expect(within(rows[1]).getByTestId('orders-list-warehouse')).toHaveTextContent('Склад не определён');
        expect(within(rows[1]).getByTestId('orders-list-scheme')).toHaveTextContent('Экспресс');
        expect(within(rows[2]).getByTestId('orders-list-warehouse')).toHaveTextContent('—');
        expect(within(rows[3]).getByTestId('orders-list-warehouse')).toHaveTextContent('Склад на Ленина');
    });

    it('offers the schemes Express and FBY and the filter «Только тестовые» / «Без тестовых», kept in the address with a chip that removes it', () => {
        setPageUrl('/orders?test=only&scheme=express');
        renderPageWith(Page, pageData({ filter: { test: 'only', scheme: 'express' }, orders: [] }));

        const schemes = within(screen.getByTestId('orders-filter-scheme')).getAllByRole('option');
        expect(schemes.map((option) => option.textContent)).toEqual(['Все', 'FBS', 'FBO', 'Экспресс', 'FBY']);
        const tests = within(screen.getByTestId('orders-filter-test')).getAllByRole('option');
        expect(tests.map((option) => option.textContent)).toEqual(['Все', 'Только тестовые', 'Без тестовых']);
        expect(screen.getByTestId('orders-filter-test')).toHaveValue('only');
        expect(screen.getByTestId('orders-filter-chip-test')).toHaveTextContent('Тестовые заказы: Только тестовые');
        expect(screen.getByTestId('orders-filter-chip-test')).toHaveAttribute('href', '/orders?scheme=express');
        expect(screen.getByTestId('orders-filter-chip-scheme')).toHaveTextContent('Схема: Экспресс');
    });

    it('offers «Требуют внимания» as a filter kept in the address, with a chip that removes it', () => {
        setPageUrl('/orders?attention=true&q=DRESS');
        renderPageWith(Page, pageData({ filter: { attention: 'true', q: 'DRESS' }, orders: [] }));

        expect(screen.getByTestId('orders-filter-attention')).toBeChecked();
        expect(screen.getByTestId('orders-filter-chip-attention')).toHaveTextContent('Требуют внимания');
        expect(screen.getByTestId('orders-filter-chip-attention')).toHaveAttribute('href', '/orders?q=DRESS');
        expect(screen.getByTestId('orders-empty-filtered')).toHaveTextContent('«Требуют внимания»');
    });

    it('opens the details of the order the address names beside the list', () => {
        setPageUrl('/orders?order=101');
        renderPageWith(Page, pageData({ opened: orderDetailsOf() }));

        expect(screen.getByTestId('order-details-panel')).toHaveTextContent('Заказ 4455667788');
        expect(screen.getByTestId('order-details')).toBeInTheDocument();
        expect(screen.getByTestId('order-lines-1-article')).toHaveTextContent('DRESS-RED-42');
    });

    it('words the refusal when the list cannot be read', () => {
        renderPageWith(Page, pageData({ refusal: 'internal' }));

        expect(screen.getByTestId('orders-refusal')).toHaveTextContent('В программе wyo произошла ошибка.');
        expect(screen.getByTestId('orders-retry')).toBeInTheDocument();
    });
});
