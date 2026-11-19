import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ShopList from './ShopList.svelte';
import type { TShop } from './types';

const shop = (change: Partial<TShop> = {}): TShop => ({
    number: '21001',
    name: 'Одежда на Ленина',
    scheme: 'FBS',
    apiOff: false,
    ...change
});

/** The shops of one account: each with its name, its number and its scheme, and the warning of a shop whose API the marketplace switched off. */
describe('ShopList', () => {
    it('lists every shop with its name, its number and its scheme in the schemes’ own words, under a heading with a «?»', () => {
        render(ShopList, {
            props: {
                lang: 'ru',
                shops: [shop(), shop({ number: '21002', name: 'Склад курьеров', scheme: 'FBY' })],
                testId: 'shops'
            }
        });

        expect(screen.getByTestId('shops-heading')).toHaveTextContent('Магазины аккаунта');
        expect(screen.getByTestId('shops-help-button')).toBeInTheDocument();
        const rows = within(screen.getByTestId('shops-list')).getAllByRole('listitem');
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent('Одежда на Ленина');
        expect(rows[0]).toHaveTextContent('номер 21001');
        expect(within(rows[0]).getByTestId('shops-scheme-21001')).toHaveTextContent('FBS');
        expect(within(rows[1]).getByTestId('shops-scheme-21002')).toHaveTextContent('FBY');
        expect(screen.queryByTestId('shops-api-off-21001')).toBeNull();
    });

    it('says «API магазина выключен» with a «?» beside the shop whose API the marketplace switched off, and only beside it', () => {
        render(ShopList, {
            props: {
                lang: 'ru',
                shops: [shop(), shop({ number: '21002', name: 'Склад курьеров', apiOff: true })],
                testId: 'shops'
            }
        });

        expect(screen.getByTestId('shops-api-off-21002')).toHaveTextContent('API магазина выключен');
        expect(screen.getByTestId('shops-api-off-21002')).toHaveClass('bg-warn-50');
        expect(screen.getByTestId('shops-api-off-21002-help-button').getAttribute('aria-label')).toContain(
            'wyo не читает его заказы'
        );
        expect(screen.queryByTestId('shops-api-off-21001')).toBeNull();
    });

    it('writes a scheme the interface has no words for as the marketplace wrote it', () => {
        render(ShopList, { props: { lang: 'ru', shops: [shop({ scheme: 'NEWSCHEME' })], testId: 'shops' } });

        expect(screen.getByTestId('shops-scheme-21001')).toHaveTextContent('NEWSCHEME');
        expect(screen.queryByTestId('shops-scheme-21001-help-button')).toBeNull();
    });
});
