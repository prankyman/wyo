import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import OrderStateBadge from './OrderStateBadge.svelte';

/** The connector's own state of an order, the same for every marketplace, in the tone of the state and with its «?». */
describe('OrderStateBadge', () => {
    it.each([
        ['new', undefined, 'Новый', 'bg-brand-50'],
        ['assembling', undefined, 'На сборке', 'bg-wait-50'],
        ['in_delivery', undefined, 'В доставке', 'bg-wait-50'],
        ['received', undefined, 'Получен покупателем', 'bg-ok-50'],
        ['cancelled', undefined, 'Отменён', 'bg-ink-100'],
        ['cancelled', 'buyer', 'Отменён покупателем', 'bg-ink-100'],
        ['cancelled', 'seller', 'Отменён продавцом', 'bg-ink-100'],
        ['cancelled', 'marketplace', 'Отменён маркетплейсом', 'bg-ink-100']
    ] as const)('draws %s (cancelled by %s) as «%s»', (state, cancelledBy, label, toneClass) => {
        render(OrderStateBadge, {
            props: { lang: 'ru', state, cancelledBy, marketplaceStatus: {}, testId: 'order-state' }
        });

        expect(screen.getByTestId('order-state')).toHaveTextContent(label);
        expect(screen.getByTestId('order-state')).toHaveClass(toneClass);
        expect(screen.getByTestId('order-state-help-button')).toBeInTheDocument();
    });

    it('shows the marketplace’s own words for a state the table does not hold, in the attention tone', () => {
        render(OrderStateBadge, {
            props: {
                lang: 'ru',
                state: 'unknown',
                marketplaceStatus: { seller: 'complete', marketplace: 'lost_somewhere' },
                testId: 'order-state'
            }
        });

        expect(screen.getByTestId('order-state')).toHaveTextContent('complete / lost_somewhere');
        expect(screen.getByTestId('order-state')).toHaveClass('bg-warn-50');
        expect(screen.getByTestId('order-state-help-button').getAttribute('aria-label')).toContain(
            'словами, которых wyo пока не знает'
        );
    });
});
