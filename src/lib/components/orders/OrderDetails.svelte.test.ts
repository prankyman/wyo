import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { exchangeDetailsOf } from '$test/support/external-api';
import { MARKETPLACE, MARKETPLACE_NAME, orderOf } from '$test/support/marketplaces';
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
                    ]
                }
            }
        });

        expect(screen.getByTestId('order-mark')).toHaveTextContent(MARKETPLACE_NAME);
        expect(screen.getByTestId('order-state')).toHaveTextContent('На сборке');
        expect(screen.getByTestId('order-scheme')).toHaveTextContent('FBS');
        expect(screen.getByTestId('order-facts-account')).toHaveTextContent('Магазин одежды');
        expect(screen.getByTestId('order-facts-article')).toHaveTextContent('DRESS-RED-42');
        expect(screen.getByTestId('order-facts-barcodes')).toHaveTextContent('2040000000017');
        expect(screen.getByTestId('order-facts-kind')).toHaveTextContent('Платья');
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
                    ]
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
                details: { order: orderOf({ state: 'cancelled', cancelledBy: 'seller' }), events: [] }
            }
        });

        expect(screen.getByTestId('order-state')).toHaveTextContent('Отменён');
        expect(screen.queryByTestId('order-deadline')).toBeNull();
    });

    it('leaves out what an FBO order does not have: no deadline, no basket, no product it was not told', () => {
        render(OrderDetails, {
            props: {
                lang: 'ru',
                details: {
                    order: orderOf({
                        scheme: 'fbo',
                        deadlineAt: undefined,
                        basket: undefined,
                        kind: undefined,
                        brand: undefined,
                        size: undefined,
                        barcodes: []
                    }),
                    events: []
                }
            }
        });

        expect(screen.getByTestId('order-scheme')).toHaveTextContent('FBO');
        expect(screen.queryByTestId('order-deadline')).toBeNull();
        expect(screen.queryByTestId('order-facts-basket')).toBeNull();
        expect(screen.queryByTestId('order-facts-kind')).toBeNull();
        expect(screen.queryByTestId('order-facts-barcodes')).toBeNull();
        expect(screen.queryByTestId('order-history')).toBeNull();
    });

    it('draws the exchange with the fulfilment centre for an FBS order that has one, and not for an order that has none', () => {
        const { unmount } = render(OrderDetails, {
            props: {
                lang: 'ru',
                details: { order: orderOf(), events: [], exchange: exchangeDetailsOf({ state: 'label_sent' }) }
            }
        });

        expect(screen.getByTestId('order-exchange')).toBeInTheDocument();
        expect(screen.getByTestId('exchange-state')).toHaveTextContent('этикетка передана');
        unmount();

        render(OrderDetails, { props: { lang: 'ru', details: { order: orderOf(), events: [] } } });
        expect(screen.queryByTestId('order-exchange')).toBeNull();
    });
});
