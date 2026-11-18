import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { exchangeDetailsOf } from '$test/support/external-api';
import { lineOf, MARKETPLACE, MARKETPLACE_NAME, orderDetailsOf, orderOf } from '$test/support/marketplaces';
import OrderDetails from './OrderDetails.svelte';

/** An order's details beside the list: every field, the marketplace's own words, and the history of its states. */
describe('OrderDetails', () => {
    it('draws every field the order has, the marketplace’s words for its state, and its history, oldest first', () => {
        render(OrderDetails, {
            props: {
                lang: 'ru',
                marketplace: MARKETPLACE,
                details: {
                    order: orderOf({
                        state: 'assembling',
                        marketplaceStatus: { seller: 'confirm', marketplace: 'waiting' }
                    }),
                    events: [
                        {
                            at: '2026-11-17T12:00:00',
                            state: 'new',
                            marketplaceStatus: { seller: 'new', marketplace: 'waiting' },
                            source: 'orders.new'
                        },
                        {
                            at: '2026-11-17T14:00:00',
                            state: 'assembling',
                            marketplaceStatus: { seller: 'confirm', marketplace: 'waiting' },
                            source: 'orders.states'
                        }
                    ],
                    parts: []
                }
            }
        });

        expect(screen.getByTestId('order-mark')).toHaveTextContent(MARKETPLACE_NAME);
        expect(screen.getByTestId('order-state')).toHaveTextContent('На сборке');
        expect(screen.getByTestId('order-scheme')).toHaveTextContent('FBS');
        expect(screen.getByTestId('order-facts-account')).toHaveTextContent('Магазин одежды');
        expect(screen.getByTestId('order-lines-1-article')).toHaveTextContent('DRESS-RED-42');
        expect(screen.getByTestId('order-lines-1-barcodes')).toHaveTextContent('2040000000017');
        expect(screen.getByTestId('order-lines-1-kind')).toHaveTextContent('Платья');
        expect(screen.getByTestId('order-facts-basket')).toHaveTextContent('1234567');
        expect(screen.getByTestId('order-facts-marketplaceStatus')).toHaveTextContent('confirm / waiting');
        expect(screen.getByTestId('order-deadline')).toHaveAttribute('data-tone');
        const events = within(screen.getByTestId('order-history')).getAllByRole('listitem');
        expect(events).toHaveLength(2);
        expect(events[0]).toHaveTextContent('Новый');
        expect(events[0]).toHaveTextContent('чтение новых заказов');
        expect(events[1]).toHaveTextContent('проверка состояний заказов');
    });

    it('says in the history who cancelled the order, with the same words as the list', () => {
        render(OrderDetails, {
            props: {
                lang: 'ru',
                details: {
                    order: orderOf({ state: 'cancelled', cancelledBy: 'buyer' }),
                    events: [
                        {
                            at: '2026-11-17T14:00:00',
                            state: 'cancelled',
                            cancelledBy: 'buyer',
                            marketplaceStatus: { seller: 'cancel', marketplace: 'canceled_by_client' },
                            source: 'orders.states'
                        }
                    ],
                    parts: []
                }
            }
        });

        expect(within(screen.getByTestId('order-history')).getByTestId('order-history-state')).toHaveTextContent(
            'Отменён покупателем'
        );
    });

    it('leaves out the deadline of an order no longer waiting to be handed over', () => {
        render(OrderDetails, {
            props: {
                lang: 'ru',
                details: orderDetailsOf({ order: orderOf({ state: 'cancelled', cancelledBy: 'seller' }) })
            }
        });

        expect(screen.getByTestId('order-state')).toHaveTextContent('Отменён');
        expect(screen.queryByTestId('order-deadline')).toBeNull();
    });

    it('leaves out what an FBO order does not have: no deadline, no basket, no product it was not told', () => {
        render(OrderDetails, {
            props: {
                lang: 'ru',
                details: orderDetailsOf({
                    order: orderOf({
                        scheme: 'fbo',
                        deadlineAt: undefined,
                        basket: undefined,
                        lines: [lineOf({ kind: undefined, brand: undefined, size: undefined, barcodes: [] })]
                    })
                })
            }
        });

        expect(screen.getByTestId('order-scheme')).toHaveTextContent('FBO');
        expect(screen.queryByTestId('order-deadline')).toBeNull();
        expect(screen.queryByTestId('order-facts-basket')).toBeNull();
        expect(screen.queryByTestId('order-lines-1-kind')).toBeNull();
        expect(screen.queryByTestId('order-lines-1-barcodes')).toBeNull();
        expect(screen.queryByTestId('order-history')).toBeNull();
    });

    it('draws the exchange with the fulfilment centre for an FBS order that has one, and not for an order that has none', () => {
        const { unmount } = render(OrderDetails, {
            props: {
                lang: 'ru',
                details: orderDetailsOf({ exchange: exchangeDetailsOf({ state: 'label_sent' }) })
            }
        });

        expect(screen.getByTestId('order-exchange')).toBeInTheDocument();
        expect(screen.getByTestId('exchange-state')).toHaveTextContent('этикетка передана');
        unmount();

        render(OrderDetails, { props: { lang: 'ru', details: orderDetailsOf() } });
        expect(screen.queryByTestId('order-exchange')).toBeNull();
    });

    it('draws every product of an order of several, each with its quantity, through the lines block', () => {
        render(OrderDetails, {
            props: {
                lang: 'ru',
                details: orderDetailsOf({
                    order: orderOf({
                        lines: [lineOf({ quantity: 2 }), lineOf({ article: 'SHIRT-BLUE-M', quantity: 1 })]
                    })
                })
            }
        });

        expect(screen.getByTestId('order-lines')).toBeInTheDocument();
        expect(screen.getByTestId('order-lines-1-quantity')).toHaveTextContent('2 единицы товара');
        expect(screen.getByTestId('order-lines-2-article')).toHaveTextContent('SHIRT-BLUE-M');
        expect(screen.queryByTestId('order-facts-article')).toBeNull();
    });

    it('names the order a split order came from, and the orders a split order was split into', () => {
        const { unmount } = render(OrderDetails, {
            props: { lang: 'ru', details: orderDetailsOf({ order: orderOf({ parentNumber: '89491381-0072-1' }) }) }
        });

        expect(screen.getByTestId('order-facts-parent')).toHaveTextContent('89491381-0072-1');
        expect(screen.queryByTestId('order-facts-parts')).toBeNull();
        unmount();

        render(OrderDetails, {
            props: { lang: 'ru', details: orderDetailsOf({ parts: ['89491381-0072-2', '89491381-0072-3'] }) }
        });
        expect(screen.getByTestId('order-facts-parts')).toHaveTextContent('89491381-0072-2, 89491381-0072-3');
        expect(screen.queryByTestId('order-facts-parent')).toBeNull();
    });
});
