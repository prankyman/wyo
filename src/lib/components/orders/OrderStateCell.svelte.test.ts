import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import OrderStateCell from './OrderStateCell.svelte';

/** The state of an order in the list: the badge, and for a split order the line with its parts. */
describe('OrderStateCell', () => {
    it('draws a split order as «Отменён» with the line «Разделён на части» naming the parts, and its «?»', () => {
        render(OrderStateCell, {
            props: {
                lang: 'ru',
                order: {
                    state: 'cancelled',
                    marketplaceStatus: { seller: 'posting_split_pending', marketplace: 'cancelled_from_split_pending' },
                    parts: ['70-2', '70-3']
                },
                testId: 'order-state'
            }
        });

        expect(screen.getByTestId('order-state')).toHaveTextContent('Отменён');
        expect(screen.getByTestId('order-state-parts')).toHaveTextContent('Разделён на части: 70-2, 70-3');
        expect(screen.getByTestId('order-state-parts-help-button').getAttribute('aria-label')).toContain(
            'Товары этого заказа теперь в частях'
        );
    });

    it.each([
        ['without parts', undefined],
        ['with an empty list of parts', []]
    ])('draws an order %s as its badge alone', (_, parts) => {
        render(OrderStateCell, {
            props: {
                lang: 'ru',
                order: { state: 'cancelled', cancelledBy: 'buyer', marketplaceStatus: {}, parts },
                testId: 'order-state'
            }
        });

        expect(screen.getByTestId('order-state')).toHaveTextContent('Отменён покупателем');
        expect(screen.queryByTestId('order-state-parts')).toBeNull();
    });
});
