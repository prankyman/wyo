import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import TestOrderBadge from './TestOrderBadge.svelte';

/** The mark of an order the seller made in the marketplace's cabinet to try the walk: a badge with a «?» that says what it means. */
describe('TestOrderBadge', () => {
    it('draws «Тестовый заказ» with a «?» that says there is no buyer, no money and no place in the home page’s numbers', () => {
        render(TestOrderBadge, { props: { lang: 'ru', testId: 'test-order' } });

        expect(screen.getByTestId('test-order')).toHaveTextContent('Тестовый заказ');
        expect(screen.getByTestId('test-order')).toHaveClass('bg-brand-50');
        expect(screen.getByTestId('test-order-help-button').getAttribute('aria-label')).toBe(
            'Этот заказ создан продавцом в кабинете маркетплейса для проверки. Покупателя нет, деньги не списываются, маркетплейс не учитывает ошибки по нему. wyo показывает и передаёт его, но не включает ни в одно число на главной странице.'
        );
    });
});
