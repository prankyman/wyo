import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { lineOf } from '$test/support/marketplaces';
import type { TOrderLine } from './types';
import ProductCell from './ProductCell.svelte';

/** The product cell of the order list: the first product of the order, its quantity, and how many more products there are. */
describe('ProductCell', () => {
    it('draws the one product of an order the way the list always did, with no quantity and no «ещё»', () => {
        render(ProductCell, { props: { lang: 'ru', lines: [lineOf()], testId: 'product' } });

        expect(screen.getByTestId('product-first')).toHaveTextContent(
            'Платья · Северный ветер · размер 42 · артикул DRESS-RED-42 · штрихкод 2040000000017'
        );
        expect(screen.queryByTestId('product-quantity')).toBeNull();
        expect(screen.queryByTestId('product-more')).toBeNull();
    });

    it('names the product by its name first when the marketplace gave one, and leaves out what it did not give', () => {
        render(ProductCell, {
            props: {
                lang: 'ru',
                lines: [
                    lineOf({ name: 'Платье красное', kind: undefined, brand: undefined, size: undefined, barcodes: [] })
                ],
                testId: 'product'
            }
        });

        expect(screen.getByTestId('product-first')).toHaveTextContent(/^Платье красное · артикул DRESS-RED-42$/);
    });

    it('shows the quantity beside the product when it is above one', () => {
        render(ProductCell, { props: { lang: 'ru', lines: [lineOf({ quantity: 3 })], testId: 'product' } });

        expect(screen.getByTestId('product-quantity')).toHaveTextContent('× 3');
        expect(screen.queryByTestId('product-more')).toBeNull();
    });

    it('shows the first product and counts the rest in words when the order has several', () => {
        render(ProductCell, {
            props: {
                lang: 'ru',
                lines: [
                    lineOf({ quantity: 2 }),
                    lineOf({ article: 'SHIRT-BLUE-M', quantity: 1 }),
                    lineOf({ article: 'HAT-GREY', quantity: 4 })
                ],
                testId: 'product'
            }
        });

        expect(screen.getByTestId('product-first')).toHaveTextContent('артикул DRESS-RED-42');
        expect(screen.getByTestId('product-first')).not.toHaveTextContent('SHIRT-BLUE-M');
        expect(screen.getByTestId('product-quantity')).toHaveTextContent('× 2');
        expect(screen.getByTestId('product-more')).toHaveTextContent('ещё товаров: 2');
    });

    it.each<[string, TOrderLine[]]>([
        ['an order with no product', []],
        [
            'an order whose only product the marketplace told nothing about',
            [
                lineOf({
                    productId: undefined,
                    article: undefined,
                    kind: undefined,
                    brand: undefined,
                    size: undefined,
                    barcodes: []
                })
            ]
        ]
    ])('draws a quiet dash for %s', (_case, lines) => {
        render(ProductCell, { props: { lang: 'ru', lines, testId: 'product' } });

        expect(screen.getByTestId('product')).toHaveTextContent(/^—$/);
    });
});
